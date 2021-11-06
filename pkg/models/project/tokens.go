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
	"crypto/subtle"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type AccessToken string

func (t AccessToken) EqualsTimingSafe(other AccessToken) bool {
	return subtle.ConstantTimeCompare([]byte(t), []byte(other)) == 1
}

var ErrInvalidTokenFormat = &errors.ValidationError{Msg: "invalid token format"}

func (t AccessToken) ValidateReadAndWrite() error {
	if len(t) != 22 {
		return ErrInvalidTokenFormat
	}
	for i := 0; i < 10; i++ {
		if t[i] < '0' || t[i] > '9' {
			return ErrInvalidTokenFormat
		}
	}
	for i := 10; i < 22; i++ {
		if t[i] < 'a' || t[i] > 'z' {
			return ErrInvalidTokenFormat
		}
	}
	return nil
}

func (t AccessToken) ValidateReadOnly() error {
	if len(t) != 12 {
		return ErrInvalidTokenFormat
	}
	for i := 0; i < 12; i++ {
		if t[i] < 'a' || t[i] > 'z' {
			return ErrInvalidTokenFormat
		}
	}
	return nil
}

type Tokens struct {
	ReadOnly           AccessToken `json:"readOnly" bson:"readOnly"`
	ReadAndWrite       AccessToken `json:"readAndWrite" bson:"readAndWrite"`
	ReadAndWritePrefix string      `json:"readAndWritePrefix" bson:"readAndWritePrefix"`
}
