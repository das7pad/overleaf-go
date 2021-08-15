// Golang port of the Overleaf notifications service
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

package notifications

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type Notification struct {
	Id             primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
	Key            string             `json:"key" bson:"key"`
	UserId         primitive.ObjectID `json:"user_id" bson:"user_id"`
	Expires        time.Time          `json:"expires,omitempty" bson:"expires,omitempty"`
	TemplateKey    string             `json:"templateKey,omitempty" bson:"templateKey,omitempty"`
	MessageOptions *bson.M            `json:"messageOpts,omitempty" bson:"messageOpts,omitempty"`
}

type Manager interface {
	GetUserNotifications(
		ctx context.Context,
		userId primitive.ObjectID,
	) ([]Notification, error)

	AddNotification(
		ctx context.Context,
		userId primitive.ObjectID,
		notification Notification,
		forceCreate bool,
	) error

	RemoveNotificationById(
		ctx context.Context,
		userId primitive.ObjectID,
		notificationId primitive.ObjectID,
	) error

	RemoveNotificationByKey(
		ctx context.Context,
		userId primitive.ObjectID,
		notificationKey string,
	) error

	RemoveNotificationByKeyOnly(
		ctx context.Context,
		notificationKey string,
	) error
}

func New(db *mongo.Database) Manager {
	return &manager{
		c: db.Collection("notifications"),
	}
}

type manager struct {
	c *mongo.Collection
}

func (m *manager) GetUserNotifications(ctx context.Context, userId primitive.ObjectID) ([]Notification, error) {
	notifications := make([]Notification, 0)
	c, err := m.c.Find(
		ctx,
		bson.M{
			"user_id": userId,
			"templateKey": bson.M{
				"$exists": true,
			},
		},
	)
	if err != nil {
		return nil, err
	}
	err = c.All(ctx, &notifications)
	if err != nil {
		return nil, err
	}
	return notifications, nil
}

func (m *manager) AddNotification(ctx context.Context, userId primitive.ObjectID, notification Notification, forceCreate bool) error {
	if notification.Key == "" {
		return &errors.ValidationError{
			Msg: "cannot add notification: missing key",
		}
	}

	filter := bson.M{
		"user_id": userId,
		"key":     notification.Key,
	}
	if !forceCreate {
		if n, err := m.c.CountDocuments(ctx, filter); err != nil {
			return err
		} else if n != 0 {
			return nil
		}
	}

	notification.UserId = userId
	_, err := m.c.UpdateOne(
		ctx,
		filter,
		bson.M{"$set": notification},
		options.Update().SetUpsert(true),
	)
	return err
}

func (m *manager) RemoveNotificationById(ctx context.Context, userId primitive.ObjectID, notificationId primitive.ObjectID) error {
	filter := bson.M{
		"user_id": userId,
		"_id":     notificationId,
	}
	update := bson.M{
		"$unset": bson.M{
			"templateKey": true,
			"messageOpts": true,
		},
	}
	_, err := m.c.UpdateOne(ctx, filter, update)
	return err
}

func (m *manager) RemoveNotificationByKey(ctx context.Context, userId primitive.ObjectID, notificationKey string) error {
	if notificationKey == "" {
		return &errors.ValidationError{
			Msg: "cannot remove notification by key: missing notificationKey",
		}
	}

	filter := bson.M{
		"user_id": userId,
		"key":     notificationKey,
	}
	update := bson.M{
		"$unset": bson.M{
			"templateKey": true,
		},
	}
	_, err := m.c.UpdateOne(ctx, filter, update)
	return err
}

func (m *manager) RemoveNotificationByKeyOnly(ctx context.Context, notificationKey string) error {
	if notificationKey == "" {
		return &errors.ValidationError{
			Msg: "cannot remove notification by key only: missing notificationKey",
		}
	}

	filter := bson.M{
		"key": notificationKey,
	}
	update := bson.M{
		"$unset": bson.M{
			"templateKey": true,
		},
	}
	_, err := m.c.UpdateOne(ctx, filter, update)
	return err
}
