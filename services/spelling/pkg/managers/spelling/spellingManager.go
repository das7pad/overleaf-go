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

package spelling

import (
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/pkg/models/learnedWords"
	"github.com/das7pad/overleaf-go/services/spelling/pkg/managers/spelling/internal/aspell"
	"github.com/das7pad/overleaf-go/services/spelling/pkg/types"
)

type Manager interface {
	aspellManager
	learnedWordsManager
}

func New(options *types.Options, db *mongo.Database) (Manager, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}

	a, err := aspell.New(options.LRUSize)
	if err != nil {
		return nil, err
	}
	return &manager{
		aspellManager:       a,
		learnedWordsManager: learnedWords.New(db),
	}, nil
}

type learnedWordsManager learnedWords.Manager
type aspellManager aspell.Manager

type manager struct {
	aspellManager
	learnedWordsManager
}
