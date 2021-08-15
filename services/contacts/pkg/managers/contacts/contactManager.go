// Golang port of the Overleaf contacts service
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

package contacts

import (
	"context"
	"fmt"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const NoLimit = -1

type ContactManager interface {
	GetContacts(
		ctx context.Context,
		userId primitive.ObjectID,
		limit int,
	) ([]primitive.ObjectID, error)

	AddContacts(
		ctx context.Context,
		userId, contactId primitive.ObjectID,
	) error
}

func NewContactManager(db *mongo.Database) ContactManager {
	contactsCollection := db.Collection("contacts")
	return &contactManager{contactsCollection: contactsCollection}
}

type userIdFilter struct {
	UserId primitive.ObjectID `bson:"user_id"`
}

type contactDetails struct {
	Connections int                `bson:"n"`
	LastTouched primitive.DateTime `bson:"ts"`
}

type contactsDocument struct {
	Contacts map[string]contactDetails `bson:"contacts"`
}

type contactManager struct {
	contactsCollection *mongo.Collection
}

type contact struct {
	UserId  string
	Details contactDetails
}

func (c contact) IsPreferredOver(other contact) bool {
	a := c.Details
	b := other.Details
	if a.Connections > b.Connections {
		return true
	} else if a.Connections < b.Connections {
		return false
	} else if a.LastTouched > b.LastTouched {
		return true
	} else if a.LastTouched < b.LastTouched {
		return false
	} else {
		return false
	}
}

func (cm *contactManager) GetContacts(
	ctx context.Context,
	userId primitive.ObjectID,
	limit int,
) ([]primitive.ObjectID, error) {
	entry := contactsDocument{Contacts: map[string]contactDetails{}}
	err := cm.
		contactsCollection.
		FindOne(ctx, userIdFilter{UserId: userId}).
		Decode(&entry)
	if err != nil && err != mongo.ErrNoDocuments {
		return nil, err
	}

	contacts := make([]contact, 0)
	for contactId, details := range entry.Contacts {
		contacts = append(contacts, contact{
			UserId:  contactId,
			Details: details,
		})
	}
	sort.Slice(contacts, func(i, j int) bool {
		return contacts[i].IsPreferredOver(contacts[j])
	})

	responseSize := len(contacts)
	if limit != NoLimit && responseSize > limit {
		responseSize = limit
	}
	contactIds := make([]primitive.ObjectID, responseSize)
	for i := 0; i < responseSize; i++ {
		id, err := primitive.ObjectIDFromHex(contacts[i].UserId)
		if err != nil {
			return nil, err
		}
		contactIds[i] = id
	}

	return contactIds, nil
}

func prepareTouchContactOneWay(
	userId, contactId primitive.ObjectID,
) mongo.WriteModel {
	now := primitive.NewDateTimeFromTime(time.Now())
	return mongo.NewUpdateOneModel().
		SetFilter(userIdFilter{UserId: userId}).
		SetUpdate(bson.M{
			"$inc": bson.M{
				fmt.Sprintf("contacts.%s.n", contactId.Hex()): 1,
			},
			"$set": bson.M{
				fmt.Sprintf("contacts.%s.ts", contactId.Hex()): now,
			},
		}).
		SetUpsert(true)
}

func (cm *contactManager) AddContacts(
	ctx context.Context,
	userId, contactId primitive.ObjectID,
) error {
	_, err := cm.contactsCollection.BulkWrite(
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
