// Golang port of Overleaf
// Copyright (C) 2021-2022 Jakob Ackermann <das7pad@outlook.com>
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

package types

import (
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type RenameDocUpdate struct {
	DocId   sharedTypes.UUID
	NewPath sharedTypes.PathName
}

func (r RenameDocUpdate) Validate() error {
	if r.NewPath == "" {
		return &errors.ValidationError{Msg: "missing new path"}
	}
	return nil
}

type RenameDocUpdates []RenameDocUpdate

func (p RenameDocUpdates) Validate() error {
	if len(p) == 0 {
		return &errors.ValidationError{Msg: "missing updates"}
	}
	for _, update := range p {
		if err := update.Validate(); err != nil {
			return err
		}
	}
	return nil
}
