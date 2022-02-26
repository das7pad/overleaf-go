// Golang port of Overleaf
// Copyright (C) 2022 Jakob Ackermann <das7pad@outlook.com>
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

package _0220126233000_users_signUpDate_typo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/cmd/migrate/internal/register"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
)

const (
	worker    = 5
	queueSize = worker * 2
)

type withMisspelledSignUpDate struct {
	user.IdField `edgedb:"inline"`
	SignUpDate   time.Time `edgedb:"SignUpDate"`
}

func Migrate(ctx context.Context, db *mongo.Database) error {
	c := db.Collection("users")

	eg, pCtx := errgroup.WithContext(ctx)
	r, errF := c.Find(pCtx, bson.M{
		"SignUpDate": bson.M{
			"$exists": true,
		},
	}, options.Find().
		SetBatchSize(100).
		SetProjection(bson.M{
			"_id":        true,
			"SignUpDate": true,
		}),
	)
	if errF != nil {
		return errors.Tag(errF, "cannot get cursor")
	}

	queue := make(chan *withMisspelledSignUpDate, queueSize)
	for i := 0; i < worker; i++ {
		eg.Go(func() error {
			for next := range queue {
				u := bson.M{
					"$set": user.SignUpDateField{
						SignUpDate: next.SignUpDate,
					},
					"$unset": bson.M{
						"SignUpDate": true,
					},
				}
				if _, err := c.UpdateOne(pCtx, next.IdField, u); err != nil {
					return errors.Tag(
						err, "cannot rename field for: "+next.Id.String(),
					)
				}
			}
			return nil
		})
	}

	eg.Go(func() error {
		defer close(queue)
		for r.Next(ctx) {
			next := &withMisspelledSignUpDate{}
			if err := r.Decode(next); err != nil {
				return errors.Tag(err, "cannot decode next")
			}
			queue <- next
		}
		if err := r.Err(); err != nil {
			return errors.Tag(err, "cannot iter cursor")
		}
		return nil
	})
	return eg.Wait()
}

func init() {
	register.Migration("20220126233000_users_signUpDate_typo", Migrate)
}
