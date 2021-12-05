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

package thread

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ResolvedDetails struct {
	At       time.Time          `bson:"ts"`
	ByUserId primitive.ObjectID `bson:"user_id"`
}

type Room struct {
	Id        primitive.ObjectID  `bson:"_id"`
	ProjectId primitive.ObjectID  `bson:"project_id"`
	ThreadId  *primitive.ObjectID `bson:"thread_id,omitempty"`
	Resolved  *ResolvedDetails    `bson:"resolved,omitempty"`
}

type Manager interface {
	FindOrCreateThread(
		ctx context.Context,
		projectId primitive.ObjectID,
		threadId *primitive.ObjectID,
	) (*Room, error)

	FindAllThreadRooms(
		ctx context.Context,
		projectId primitive.ObjectID,
	) ([]Room, error)

	ResolveThread(
		ctx context.Context,
		projectId, threadId, userId primitive.ObjectID,
	) error

	ReopenThread(
		ctx context.Context,
		projectId, threadId primitive.ObjectID,
	) error

	DeleteThread(
		ctx context.Context,
		projectId, threadId primitive.ObjectID,
	) (*primitive.ObjectID, error)

	DeleteProjectThreads(ctx context.Context, projectId primitive.ObjectID) error
}

func NewThreadManager(db *mongo.Database) Manager {
	return &manager{
		roomsCollection: db.Collection("rooms"),
	}
}

type manager struct {
	roomsCollection *mongo.Collection
}

func (m *manager) FindOrCreateThread(
	ctx context.Context,
	projectId primitive.ObjectID,
	threadId *primitive.ObjectID,
) (*Room, error) {

	var query, roomDetails bson.M
	if threadId == nil {
		query = bson.M{
			"project_id": projectId,
			"thread_id": bson.M{
				"$exists": false,
			},
		}
		roomDetails = bson.M{
			"project_id": projectId,
		}
	} else {
		query = bson.M{
			"project_id": projectId,
			"thread_id":  threadId,
		}
		roomDetails = bson.M{
			"project_id": projectId,
			"thread_id":  threadId,
		}
	}
	update := bson.M{
		"$set": roomDetails,
	}

	result := m.roomsCollection.FindOneAndUpdate(
		ctx,
		query,
		update,
		options.FindOneAndUpdate().
			SetUpsert(true).
			SetReturnDocument(options.After),
	)
	var thread Room
	err := result.Decode(&thread)
	return &thread, err
}

func (m *manager) FindAllThreadRooms(
	ctx context.Context,
	projectId primitive.ObjectID,
) ([]Room, error) {
	query := bson.M{
		"project_id": projectId,
		"thread_id": bson.M{
			"$exists": true,
		},
	}
	projection := bson.M{
		"thread_id": 1,
		"resolved":  1,
	}
	c, err := m.roomsCollection.Find(
		ctx,
		query,
		options.Find().SetProjection(projection),
	)
	if err != nil {
		return nil, err
	}
	out := make([]Room, 0)
	err = c.All(ctx, &out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (m *manager) ResolveThread(
	ctx context.Context,
	projectId, threadId, userId primitive.ObjectID,
) error {
	query := bson.M{
		"project_id": projectId,
		"thread_id":  threadId,
	}
	update := bson.M{
		"$set": bson.M{
			"resolved": bson.M{
				"user_id": userId,
				"ts":      time.Now(),
			},
		},
	}
	_, err := m.roomsCollection.UpdateOne(ctx, query, update)
	return err
}

func (m *manager) ReopenThread(
	ctx context.Context,
	projectId, threadId primitive.ObjectID,
) error {
	query := bson.M{
		"project_id": projectId,
		"thread_id":  threadId,
	}
	update := bson.M{
		"$unset": bson.M{
			"resolved": true,
		},
	}
	_, err := m.roomsCollection.UpdateOne(ctx, query, update)
	return err
}

func (m *manager) DeleteThread(
	ctx context.Context,
	projectId, threadId primitive.ObjectID,
) (*primitive.ObjectID, error) {
	thread, err := m.FindOrCreateThread(ctx, projectId, &threadId)
	if err != nil {
		return nil, err
	}
	query := bson.M{
		"_id": thread.Id,
	}
	_, err = m.roomsCollection.DeleteOne(ctx, query)
	return &thread.Id, err
}

func (m *manager) DeleteProjectThreads(ctx context.Context, projectId primitive.ObjectID) error {
	q := bson.M{
		"project_id": projectId,
	}
	if _, err := m.roomsCollection.DeleteMany(ctx, q); err != nil {
		return err
	}
	return nil
}
