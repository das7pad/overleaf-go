// Golang port of Overleaf
// Copyright (C) 2021-2023 Jakob Ackermann <das7pad@outlook.com>
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

package email

import (
	"net/mail"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Identity struct {
	Address     sharedTypes.Email `json:"address"`
	DisplayName string            `json:"display_name"`
}

func (i *Identity) String() string {
	a := mail.Address{
		Address: string(i.Address),
		Name:    i.DisplayName,
	}
	return a.String()
}

func (i *Identity) Validate() error {
	if i == nil {
		return errors.New("missing identity")
	}
	return i.Address.Validate()
}
