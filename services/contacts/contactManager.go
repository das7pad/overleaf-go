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

package main

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ContactManager interface {
	GetContacts(
		ctx context.Context,
		userId primitive.ObjectID,
	) (map[string]ContactDetails, error)

	TouchContact(
		ctx context.Context,
		userId, contactId primitive.ObjectID,
	) error
}

func NewContactManager(contactsCollection mongo.Collection) ContactManager {
	return &contactManager{contactsCollection: contactsCollection}
}

type userIdFilter struct {
	UserId primitive.ObjectID `bson:"user_id"`
}

type ContactDetails struct {
	Connections int                `bson:"n"`
	LastTouched primitive.DateTime `bson:"ts"`
}

type contactsDocument struct {
	Contacts map[string]ContactDetails `bson:"contacts"`
}

type contactManager struct {
	contactsCollection mongo.Collection
}

func (cm *contactManager) GetContacts(
	ctx context.Context,
	userId primitive.ObjectID,
) (map[string]ContactDetails, error) {
	contacts := contactsDocument{Contacts: map[string]ContactDetails{}}
	err := cm.
		contactsCollection.
		FindOne(ctx, userIdFilter{UserId: userId}).
		Decode(&contacts)
	if err != nil && err != mongo.ErrNoDocuments {
		return nil, err
	}
	return contacts.Contacts, nil
}

func (cm *contactManager) TouchContact(
	ctx context.Context,
	userId, contactId primitive.ObjectID,
) error {
	now := primitive.NewDateTimeFromTime(time.Now())
	_, err := cm.contactsCollection.UpdateOne(
		ctx,
		userIdFilter{UserId: userId},
		bson.M{
			"$inc": bson.M{
				fmt.Sprintf("contacts.%s.n", contactId.Hex()): 1,
			},
			"$set": bson.M{
				fmt.Sprintf("contacts.%s.ts", contactId.Hex()): now,
			},
		},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return err
	}
	return nil
}
