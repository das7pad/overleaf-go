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

package deletedProject

import (
	"context"
	"time"

	"github.com/edgedb/edgedb-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type Manager interface {
	Expire(ctx context.Context, projectId edgedb.UUID) error
	GetExpired(ctx context.Context, age time.Duration) (<-chan edgedb.UUID, error)
}

func New(db *mongo.Database) Manager {
	c := db.Collection("deletedProjects")
	cSlow := db.Collection("deletedProjects", options.Collection().
		SetReadPreference(readpref.SecondaryPreferred()),
	)
	return &manager{
		c:     c,
		cSlow: cSlow,
	}
}

type manager struct {
	c     *mongo.Collection
	cSlow *mongo.Collection
}

func rewriteMongoError(err error) error {
	if err == mongo.ErrNoDocuments {
		return &errors.NotFoundError{}
	}
	return err
}

func (m *manager) Expire(ctx context.Context, projectId edgedb.UUID) error {
	q := bson.M{
		"deleterData.deletedProjectId": projectId,
	}
	u := bson.M{
		"$set": bson.M{
			"project":                      nil,
			"deleterData.deleterIpAddress": "",
		},
	}
	_, err := m.c.UpdateOne(ctx, q, u)
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

const bufferSize = 10

func (m *manager) GetExpired(ctx context.Context, age time.Duration) (<-chan edgedb.UUID, error) {
	q := bson.M{
		"deleterData.deletedAt": bson.M{
			"$lt": time.Now().UTC().Add(-age),
		},
		"project": bson.M{
			"$ne": nil,
		},
	}
	projection := bson.M{
		"deleterData.deletedProjectId": 1,
	}
	r, errFind := m.cSlow.Find(
		ctx, q, options.Find().
			SetProjection(projection).
			SetBatchSize(bufferSize),
	)
	if errFind != nil {
		return nil, rewriteMongoError(errFind)
	}
	queue := make(chan edgedb.UUID, bufferSize)

	// Peek once into the batch, then ignore any errors during background
	//  streaming.
	if !r.Next(ctx) {
		close(queue)
		if err := r.Err(); err != nil {
			return nil, err
		}
		return queue, nil
	}
	dp := &forListing{}
	if err := r.Decode(dp); err != nil {
		close(queue)
		return nil, err
	}

	go func() {
		defer close(queue)
		queue <- dp.DeleterData.DeletedProjectId
		for r.Next(ctx) {
			if err := r.Decode(dp); err != nil {
				return
			}
			queue <- dp.DeleterData.DeletedProjectId
		}
	}()
	return queue, nil
}
