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

package contact

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/edgedb/edgedb-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type Manager interface {
	DeleteForUser(ctx context.Context, userId edgedb.UUID) error
	GetForUser(ctx context.Context, userId edgedb.UUID, contacts *[]edgedb.UUID) error
	Add(ctx context.Context, userId, contactId edgedb.UUID) error
}

func New(db *mongo.Database) Manager {
	return &manager{c: db.Collection("contacts")}
}

type manager struct {
	c *mongo.Collection
}

func (cm *manager) GetForUser(ctx context.Context, userId edgedb.UUID, contacts *[]edgedb.UUID) error {
	entry := &ContactsField{}
	err := cm.c.FindOne(ctx, UserIdField{UserId: userId}).Decode(entry)
	if err != nil && err != mongo.ErrNoDocuments {
		return err
	}

	raw := make([]*contact, 0)
	for contactId, details := range entry.Contacts {
		raw = append(raw, &contact{
			UserId:         contactId,
			contactDetails: details,
		})
	}
	sort.Slice(raw, func(i, j int) bool {
		return raw[i].IsPreferredOver(raw[j])
	})

	n := len(raw)
	if n > 50 {
		n = 50
	}
	contactIds := make([]edgedb.UUID, n)
	for i := 0; i < n; i++ {
		id, err2 := edgedb.ParseUUID(raw[i].UserId)
		if err2 != nil {
			return err2
		}
		contactIds[i] = id
	}
	*contacts = contactIds
	return nil
}

func prepareTouchContactOneWay(
	userId, contactId edgedb.UUID,
) mongo.WriteModel {
	now := primitive.NewDateTimeFromTime(time.Now())
	return mongo.NewUpdateOneModel().
		SetFilter(UserIdField{UserId: userId}).
		SetUpdate(bson.M{
			"$inc": bson.M{
				fmt.Sprintf("contacts.%s.n", contactId.String()): 1,
			},
			"$set": bson.M{
				fmt.Sprintf("contacts.%s.ts", contactId.String()): now,
			},
		}).
		SetUpsert(true)
}

func (cm *manager) Add(ctx context.Context, userId, contactId edgedb.UUID) error {
	_, err := cm.c.BulkWrite(
		ctx,
		[]mongo.WriteModel{
			prepareTouchContactOneWay(userId, contactId),
			prepareTouchContactOneWay(contactId, userId),
		},
	)
	if err != nil {
		return err
	}
	return nil
}

func (cm *manager) DeleteForUser(ctx context.Context, userId edgedb.UUID) error {
	q := &UserIdField{UserId: userId}
	_, err := cm.c.DeleteOne(ctx, q)
	if err != nil {
		return err
	}
	return nil
}
