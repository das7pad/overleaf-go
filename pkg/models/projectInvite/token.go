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

package projectInvite

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type Token string

func (t Token) Validate() error {
	if len(t) < 32 {
		return &errors.ValidationError{Msg: "token too short"}
	}
	return nil
}

func generateNewToken() (Token, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", errors.Tag(err, "cannot generate new token")
	}
	return Token(hex.EncodeToString(b)), nil
}
