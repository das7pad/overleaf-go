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

package learnedWords

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Manager interface {
	DeleteDictionary(
		ctx context.Context,
		userId primitive.ObjectID,
	) error

	GetDictionary(
		ctx context.Context,
		userId primitive.ObjectID,
	) ([]string, error)

	LearnWord(
		ctx context.Context,
		userId primitive.ObjectID,
		word string,
	) error

	UnlearnWord(
		ctx context.Context,
		userId primitive.ObjectID,
		word string,
	) error
}

func New(db *mongo.Database) Manager {
	return &manager{
		c: db.Collection("spellingPreferences"),
	}
}

type manager struct {
	c *mongo.Collection
}

func userMatcher(userId primitive.ObjectID) bson.M {
	return bson.M{
		"token": userId.Hex(),
	}
}

func (m manager) DeleteDictionary(ctx context.Context, userId primitive.ObjectID) error {
	_, err := m.c.DeleteOne(ctx, userMatcher(userId))
	return err
}

func (m manager) GetDictionary(ctx context.Context, userId primitive.ObjectID) ([]string, error) {
	var preference spellingPreference
	err := m.c.FindOne(ctx, userMatcher(userId)).Decode(&preference)
	if err == mongo.ErrNoDocuments {
		return []string{}, nil
	}
	return preference.LearnedWords, err
}

func (m manager) LearnWord(ctx context.Context, userId primitive.ObjectID, word string) error {
	_, err := m.c.UpdateOne(ctx, userMatcher(userId), bson.M{
		"$addToSet": bson.M{
			"learnedWords": word,
		},
	}, options.Update().SetUpsert(true))
	return err
}

func (m manager) UnlearnWord(ctx context.Context, userId primitive.ObjectID, word string) error {
	_, err := m.c.UpdateOne(ctx, userMatcher(userId), bson.M{
		"$pull": bson.M{
			"learnedWords": word,
		},
	})
	return err
}
