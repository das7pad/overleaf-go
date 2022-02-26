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

package notification

import (
	"context"

	"github.com/edgedb/edgedb-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type Manager interface {
	DeleteForUser(ctx context.Context, userId edgedb.UUID) error
	GetAllForUser(ctx context.Context, userId edgedb.UUID, notifications *[]Notification) error
	Add(ctx context.Context, notification Notification, forceCreate bool) error
	RemoveById(ctx context.Context, userId edgedb.UUID, notificationId edgedb.UUID) error
	RemoveByKey(ctx context.Context, userId edgedb.UUID, notificationKey string) error
	RemoveByKeyOnly(ctx context.Context, notificationKey string) error
}

func New(db *mongo.Database) Manager {
	return &manager{
		c: db.Collection("notifications"),
	}
}

const (
	prefetchN = 10
)

type manager struct {
	c *mongo.Collection
}

func (m *manager) DeleteForUser(ctx context.Context, userId edgedb.UUID) error {
	q := &UserIdField{UserId: userId}
	_, err := m.c.DeleteMany(ctx, q)
	if err != nil {
		return err
	}
	return nil
}

func (m *manager) GetAllForUser(ctx context.Context, userId edgedb.UUID, notifications *[]Notification) error {
	c, err := m.c.Find(
		ctx,
		bson.M{
			"user_id": userId,
			"templateKey": bson.M{
				"$exists": true,
			},
		},
		options.Find().SetBatchSize(prefetchN),
	)
	if err != nil {
		return err
	}
	err = c.All(ctx, notifications)
	if err != nil {
		return err
	}
	return nil
}

func (m *manager) Add(ctx context.Context, notification Notification, forceCreate bool) error {
	if notification.Key == "" {
		return &errors.ValidationError{
			Msg: "cannot add notification: missing key",
		}
	}

	q := userIdAndKey{
		KeyField: KeyField{
			Key: notification.Key,
		},
		UserIdField: UserIdField{
			UserId: notification.UserId,
		},
	}
	if !forceCreate {
		if n, err := m.c.CountDocuments(ctx, q); err != nil {
			return err
		} else if n != 0 {
			return nil
		}
	}

	_, err := m.c.UpdateOne(
		ctx,
		q,
		bson.M{"$set": notification},
		options.Update().SetUpsert(true),
	)
	return err
}

func (m *manager) RemoveById(ctx context.Context, userId edgedb.UUID, notificationId edgedb.UUID) error {
	q := userIdAndId{
		IdField: IdField{
			Id: notificationId,
		},
		UserIdField: UserIdField{
			UserId: userId,
		},
	}
	u := bson.M{
		"$unset": bson.M{
			"templateKey": true,
			"messageOpts": true,
		},
	}
	_, err := m.c.UpdateOne(ctx, q, u)
	return err
}

func (m *manager) RemoveByKey(ctx context.Context, userId edgedb.UUID, notificationKey string) error {
	if notificationKey == "" {
		return &errors.ValidationError{
			Msg: "cannot remove notification by key: missing notificationKey",
		}
	}

	q := userIdAndKey{
		KeyField: KeyField{
			Key: notificationKey,
		},
		UserIdField: UserIdField{
			UserId: userId,
		},
	}
	u := bson.M{
		"$unset": bson.M{
			"templateKey": true,
		},
	}
	_, err := m.c.UpdateOne(ctx, q, u)
	return err
}

func (m *manager) RemoveByKeyOnly(ctx context.Context, notificationKey string) error {
	if notificationKey == "" {
		return &errors.ValidationError{
			Msg: "cannot remove notification by key only: missing notificationKey",
		}
	}

	q := KeyField{
		Key: notificationKey,
	}
	u := bson.M{
		"$unset": bson.M{
			"templateKey": true,
		},
	}
	_, err := m.c.UpdateOne(ctx, q, u)
	return err
}
