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

package _20220102130000_docVersion_stage2

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/cmd/migrate/internal/register"
	"github.com/das7pad/overleaf-go/pkg/models/doc"
)

func Migrate(ctx context.Context, db *mongo.Database) error {
	c := db.Collection("docs")
	q := bson.M{
		"version": bson.M{
			"$exists": false,
		},
	}
	u := bson.M{
		"$set": doc.VersionField{
			Version: doc.ExternalVersionTombstone,
		},
	}
	_, err := c.UpdateMany(ctx, q, u)
	if err != nil {
		return err
	}
	return nil
}

func init() {
	register.Migration("20220102130000_docVersion_stage2", Migrate)
}
