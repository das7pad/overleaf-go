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

package message

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/das7pad/overleaf-go/services/chat/pkg/types"
)

type Manager interface {
	CreateMessage(
		ctx context.Context,
		roomId, userId primitive.ObjectID,
		content string,
		timestamp float64,
	) (*types.Message, error)

	GetMessages(
		ctx context.Context,
		roomId primitive.ObjectID,
		limit int64,
		before float64,
	) ([]*types.Message, error)

	FindAllMessagesInRooms(
		ctx context.Context,
		roomIds []primitive.ObjectID,
	) ([]*types.Message, error)

	DeleteAllMessagesInRoom(
		ctx context.Context,
		roomId primitive.ObjectID,
	) error

	UpdateMessage(
		ctx context.Context,
		roomId, messageId primitive.ObjectID,
		content string,
		timestamp float64,
	) error

	DeleteMessage(
		ctx context.Context,
		roomId, messageId primitive.ObjectID,
	) error

	DeleteProjectMessages(ctx context.Context, roomIds []primitive.ObjectID) error
}

func New(db *mongo.Database) Manager {
	return &manager{
		messagesCollection: db.Collection("messages"),
	}
}

type manager struct {
	messagesCollection *mongo.Collection
}

func (m *manager) CreateMessage(
	ctx context.Context,
	roomId, userId primitive.ObjectID,
	content string,
	timestamp float64,
) (*types.Message, error) {
	message := types.Message{
		Content:   content,
		RoomId:    roomId,
		UserId:    userId,
		Timestamp: timestamp,
	}
	result, err := m.messagesCollection.InsertOne(
		ctx,
		message,
	)
	if err != nil {
		return nil, err
	}
	message.Id = result.InsertedID.(primitive.ObjectID)
	return &message, nil
}

func readAllMessages(ctx context.Context, c *mongo.Cursor) ([]*types.Message, error) {
	out := make([]*types.Message, 0)
	err := c.All(ctx, &out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (m *manager) GetMessages(
	ctx context.Context,
	roomId primitive.ObjectID,
	limit int64,
	before float64,
) ([]*types.Message, error) {
	query := bson.M{
		"room_id": roomId,
	}
	if before != 0 {
		query = bson.M{
			"room_id": roomId,
			"timestamp": bson.M{
				"$lt": before,
			},
		}
	}
	c, err := m.messagesCollection.Find(
		ctx,
		query,
		options.Find().
			SetSort(bson.M{
				"timestamp": -1,
			}).
			SetLimit(limit),
	)
	if err != nil {
		return nil, err
	}
	return readAllMessages(ctx, c)
}

func (m *manager) FindAllMessagesInRooms(
	ctx context.Context,
	roomIds []primitive.ObjectID,
) ([]*types.Message, error) {

	query := bson.M{
		"room_id": bson.M{
			"$in": roomIds,
		},
	}
	c, err := m.messagesCollection.Find(
		ctx, query,
		options.Find().SetSort(bson.M{
			"timestamp": 1,
		}),
	)
	if err != nil {
		return nil, err
	}
	return readAllMessages(ctx, c)
}

func (m *manager) DeleteAllMessagesInRoom(
	ctx context.Context,
	roomId primitive.ObjectID,
) error {
	query := bson.M{
		"room_id": roomId,
	}
	_, err := m.messagesCollection.DeleteMany(ctx, query)
	return err
}

func (m *manager) UpdateMessage(
	ctx context.Context,
	roomId, messageId primitive.ObjectID,
	content string,
	timestamp float64,
) error {
	query := bson.M{
		"room_id": roomId,
		"_id":     messageId,
	}
	update := bson.M{
		"$set": bson.M{
			"content":   content,
			"edited_at": timestamp,
		},
	}
	_, err := m.messagesCollection.UpdateOne(ctx, query, update)
	return err
}

func (m *manager) DeleteMessage(
	ctx context.Context,
	roomId, messageId primitive.ObjectID,
) error {
	query := bson.M{
		"room_id": roomId,
		"_id":     messageId,
	}
	_, err := m.messagesCollection.DeleteOne(ctx, query)
	return err
}

func (m *manager) DeleteProjectMessages(ctx context.Context, roomIds []primitive.ObjectID) error {
	query := bson.M{
		"room_id": bson.M{
			"$in": roomIds,
		},
	}
	_, err := m.messagesCollection.DeleteMany(ctx, query)
	return err
}
