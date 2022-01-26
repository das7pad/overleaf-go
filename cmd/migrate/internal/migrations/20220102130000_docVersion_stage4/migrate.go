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

package _20220102130000_docVersion_stage4

import (
	"context"
	"log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/cmd/migrate/internal/register"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/doc"
	"github.com/das7pad/overleaf-go/pkg/models/docOps"
)

const (
	worker    = 5
	queueSize = worker * 2
)

type docIdAndVersion struct {
	doc.IdField      `bson:"inline"`
	doc.VersionField `bson:"inline"`
}

func Migrate(ctx context.Context, db *mongo.Database) error {
	c := db.Collection("docs")
	cDocOps := db.Collection("docOps")

	total, errC := cDocOps.EstimatedDocumentCount(ctx)
	if errC != nil {
		return errors.Tag(errC, "cannot get estimate of total number")
	}

	eg, pCtx := errgroup.WithContext(ctx)
	r, errF := cDocOps.Find(pCtx, bson.M{}, options.Find().SetBatchSize(100))
	if errF != nil {
		return errors.Tag(errF, "cannot get cursor")
	}

	queue := make(chan *docOps.Full, queueSize)
	for i := 0; i < worker; i++ {
		eg.Go(func() error {
			for next := range queue {
				q := &docIdAndVersion{
					IdField: doc.IdField{
						Id: next.DocId,
					},
					VersionField: doc.VersionField{
						Version: doc.ExternalVersionTombstone,
					},
				}
				u := bson.M{
					"$set": doc.VersionField{Version: next.Version},
				}
				if _, err := c.UpdateOne(pCtx, q, u); err != nil {
					return errors.Tag(
						err, "cannot back-fill version: "+next.DocId.Hex(),
					)
				}
				if _, err := cDocOps.DeleteOne(pCtx, next); err != nil {
					return errors.Tag(
						err, "cannot cleanup version: "+next.DocId.Hex(),
					)
				}
			}
			return nil
		})
	}

	eg.Go(func() error {
		defer close(queue)
		i := 0
		for r.Next(ctx) {
			next := &docOps.Full{}
			if err := r.Decode(next); err != nil {
				return err
			}
			queue <- next
			i++
			if i%100 == queueSize {
				log.Printf("progess: %d/%d", i-queueSize, total)
			}
		}
		if err := r.Err(); err != nil {
			return errors.Tag(err, "cannot iter cursor")
		}
		log.Printf("progress: %d back-filled", i)
		return nil
	})
	return eg.Wait()
}

func init() {
	register.Migration("20220102130000_docVersion_stage4", Migrate)
}
