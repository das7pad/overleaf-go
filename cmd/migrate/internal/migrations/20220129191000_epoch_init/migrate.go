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

package _0220129191000_epoch_init

import (
	"context"
	"log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/cmd/migrate/internal/register"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/user"
)

func Migrate(ctx context.Context, db *mongo.Database) error {
	{
		c := db.Collection("users")
		r, err := c.UpdateMany(
			ctx, bson.M{}, bson.M{
				"$inc": user.EpochField{
					Epoch: 1,
				},
			},
		)
		if err != nil {
			return errors.Tag(err, "cannot bump all users epoch")
		}
		log.Printf("progress: %d user epochs bumped", r.ModifiedCount)
	}

	{
		c := db.Collection("projects")
		r, err := c.UpdateMany(
			ctx, bson.M{}, bson.M{
				"$inc": project.EpochField{
					Epoch: 1,
				},
			},
		)
		if err != nil {
			return errors.Tag(err, "cannot bump all projects epoch")
		}
		log.Printf("progress: %d project epochs bumped", r.ModifiedCount)
	}
	return nil
}

func init() {
	register.Migration("20220129191000_epoch_init", Migrate)
}
