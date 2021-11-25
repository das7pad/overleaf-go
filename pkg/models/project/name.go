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

package project

import (
	"github.com/das7pad/overleaf-go/pkg/errors"
)

type Name string

func (n Name) Validate() error {
	if len(n) == 0 {
		return &errors.ValidationError{Msg: "name cannot be blank"}
	}
	if len(n) > 150 {
		return &errors.ValidationError{Msg: "name is too long"}
	}
	for _, c := range n {
		switch c {
		case '/':
			return &errors.ValidationError{Msg: "name cannot contain /"}
		case '\\':
			return &errors.ValidationError{Msg: "name cannot contain \\"}
		case '\r', '\n':
			return &errors.ValidationError{
				Msg: "name cannot contain line feeds",
			}
		}
	}
	return nil
}
