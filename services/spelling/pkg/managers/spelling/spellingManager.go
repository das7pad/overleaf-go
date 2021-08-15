// Golang port of the Overleaf spelling service
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

package spelling

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/services/spelling/pkg/managers/spelling/internal/aspell"
	"github.com/das7pad/overleaf-go/services/spelling/pkg/managers/spelling/internal/learnedWords"
	"github.com/das7pad/overleaf-go/services/spelling/pkg/types"
)

type Manager interface {
	CheckWords(
		ctx context.Context,
		language string,
		words []string,
	) ([]types.Misspelling, error)

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

func New(db *mongo.Database, options *types.Options) (Manager, error) {
	lruSize := options.LRUSize
	if lruSize <= 0 {
		lruSize = 10 * 1000
	}
	a, err := aspell.New(lruSize)
	if err != nil {
		return nil, err
	}
	return &manager{
		a:  a,
		lm: learnedWords.New(db),
	}, nil
}

type manager struct {
	a  aspell.Manager
	lm learnedWords.Manager
}

func (m *manager) CheckWords(ctx context.Context, language string, words []string) ([]types.Misspelling, error) {
	if len(words) > RequestLimit {
		words = words[:RequestLimit]
	}
	return m.a.CheckWords(ctx, language, words)
}

func (m *manager) DeleteDictionary(ctx context.Context, userId primitive.ObjectID) error {
	return m.lm.DeleteDictionary(ctx, userId)
}

func (m *manager) GetDictionary(ctx context.Context, userId primitive.ObjectID) ([]string, error) {
	return m.lm.GetDictionary(ctx, userId)
}

func (m *manager) LearnWord(ctx context.Context, userId primitive.ObjectID, word string) error {
	return m.lm.LearnWord(ctx, userId, word)
}

func (m *manager) UnlearnWord(ctx context.Context, userId primitive.ObjectID, word string) error {
	return m.lm.UnlearnWord(ctx, userId, word)
}

const (
	RequestLimit = 10000
)
