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

package fileTree

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/mongoTx"
)

const retriesFileTreeOperation = 10

func (m *manager) txWithRetries(ctx context.Context, fn mongoTx.Runner) error {
	var err error
	for i := 0; i < retriesFileTreeOperation; i++ {
		err = mongoTx.For(m.db, ctx, fn)
		if errors.GetCause(err) == project.ErrVersionChanged {
			continue
		}
		break
	}
	if err != nil {
		return err
	}
	return nil
}
