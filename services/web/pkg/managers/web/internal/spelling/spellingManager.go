// Golang port of Overleaf
// Copyright (C) 2022 Jakob Ackermann <das7pad@outlook.com>
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

	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/pkg/models/learnedWords"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	GetDictionary(ctx context.Context, request *types.GetDictionaryRequest, response *types.GetDictionaryResponse) error
	LearnWord(ctx context.Context, request *types.LearnWordRequest) error
}

func New(db *mongo.Database) Manager {
	return &manager{
		lm: learnedWords.New(db),
	}
}

type manager struct {
	lm learnedWords.Manager
}

func (m *manager) GetDictionary(ctx context.Context, request *types.GetDictionaryRequest, response *types.GetDictionaryResponse) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	words, err := m.lm.GetDictionary(ctx, request.Session.User.Id)
	if err != nil {
		return err
	}
	response.Words = words
	return nil
}

func (m *manager) LearnWord(ctx context.Context, request *types.LearnWordRequest) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	return m.lm.LearnWord(ctx, request.Session.User.Id, request.Word)
}
