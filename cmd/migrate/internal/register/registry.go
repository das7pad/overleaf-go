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

package register

import (
	"context"
	"fmt"
	"sort"

	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type Runner func(ctx context.Context, db *mongo.Database) error

var _registry = make(map[string]Runner)

func rewriteMongoError(err error) error {
	if err == mongo.ErrNoDocuments {
		return &errors.NotFoundError{}
	}
	return err
}

func Migration(name string, runner Runner) {
	if _, exists := _registry[name]; exists {
		panic(fmt.Sprintf("migration %q already exists", name))
	}
	_registry[name] = runner
}

func List() []string {
	flat := make([]string, 0, len(_registry))
	for name := range _registry {
		flat = append(flat, name)
	}
	sort.Slice(flat, func(i, j int) bool {
		return flat[i] < flat[j]
	})
	return flat
}

func Run(ctx context.Context, name string, db *mongo.Database) error {
	if err := _registry[name](ctx, db); err != nil {
		return rewriteMongoError(err)
	}
	return nil
}
