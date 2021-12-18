// Golang port of Overleaf
// Copyright (C) 2021 Jakob Ackermann <das7pad@outlook.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/das7pad/overleaf-go/cmd/internal/utils"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/jwt/projectJWT"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/session"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/pkg/signedCookie"
)

func rewriteMongoError(err error) error {
	if err == mongo.ErrNoDocuments {
		return &errors.NotFoundError{}
	}
	return err
}

func setIsAdmin(ctx context.Context, c *mongo.Collection, client redis.UniversalClient, sm session.Manager, email sharedTypes.Email, isAdmin bool) error {
	u := &user.IdField{}
	err := c.FindOneAndUpdate(
		ctx,
		user.EmailField{
			Email: email,
		},
		bson.M{
			"$set": user.IsAdminField{
				IsAdmin: isAdmin,
			},
			"$inc": user.EpochField{
				Epoch: 1,
			},
		},
		options.FindOneAndUpdate().SetProjection(bson.M{"_id": true}),
	).Decode(u)
	if err != nil {
		return rewriteMongoError(err)
	}
	log.Println("clearing JWT state")
	if err = projectJWT.ClearUserField(ctx, client, u.Id); err != nil {
		return errors.Tag(err, "cannot cleanup JWT state, please retry")
	}
	log.Println("clearing sessions")
	if err = sm.DestroyAllForUser(ctx, u.Id); err != nil {
		return errors.Tag(err, "cannot cleanup sessions, please retry")
	}
	return nil
}

func promoteToAdmin(ctx context.Context, c *mongo.Collection, client redis.UniversalClient, sm session.Manager, email sharedTypes.Email) error {
	log.Printf("promoting %q to admin role", email)
	return setIsAdmin(ctx, c, client, sm, email, true)
}

func demoteFromAdmin(ctx context.Context, c *mongo.Collection, client redis.UniversalClient, sm session.Manager, email sharedTypes.Email) error {
	log.Printf("demoting %q from admin role", email)
	return setIsAdmin(ctx, c, client, sm, email, false)
}

func main() {
	emailRaw := flag.String("email", "", "users email address")
	promote := flag.Bool("promote", false, "set to promote to admin")
	demote := flag.Bool("demote", false, "set to demote from admin")
	timeout := flag.Duration("timout", 10*time.Second, "timeout for operation")
	flag.Parse()
	if *emailRaw == "" {
		fmt.Println("ERR: must set -email")
		flag.Usage()
		os.Exit(101)
	}
	email := sharedTypes.Email(*emailRaw).Normalize()
	if err := email.Validate(); err != nil {
		fmt.Println(errors.Tag(err, "ERR: invalid email address").Error())
		flag.Usage()
		os.Exit(101)
	}
	if *promote == false && *demote == false {
		fmt.Println("ERR: must set either -promote or -demote")
		flag.Usage()
		os.Exit(101)
	}

	rClient := utils.MustConnectRedis(*timeout)
	db := utils.MustConnectMongo(*timeout)
	c := db.Collection("users")

	sm := session.New(signedCookie.Options{
		Secrets: []string{"not-used"},
	}, rClient)

	ctx, done := context.WithTimeout(context.Background(), *timeout)
	defer done()
	var err error
	if *promote {
		err = promoteToAdmin(ctx, c, rClient, sm, email)
	} else {
		err = demoteFromAdmin(ctx, c, rClient, sm, email)
	}
	if err != nil {
		if errors.IsNotFoundError(err) {
			fmt.Println("user not found. make sure the user has registered.")
			os.Exit(1)
		}
		panic(err)
	}
	log.Println("done.")
}
