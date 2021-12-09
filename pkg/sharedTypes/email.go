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

package sharedTypes

import (
	"net/mail"
	"strings"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type ReversedHostname string

type Email string

func (e Email) LocalPart() string {
	s := string(e)
	idx := strings.LastIndexByte(s, '@')
	return s[:idx]
}

func (e Email) Host() string {
	s := string(e)
	idx := strings.LastIndexByte(s, '@')
	return s[idx+1:]
}

func (e Email) ReversedHostname() ReversedHostname {
	h := e.Host()
	out := []rune(h)
	n := len(out)
	for i, j := 0, n; i < n/2; i++ {
		j--
		out[i], out[j] = out[j], out[i]
	}
	return ReversedHostname(out)
}

func (e Email) Normalize() Email {
	return Email(strings.ToLower(string(e)))
}

func (e Email) Validate() error {
	_, err := mail.ParseAddress(string(e))
	if err != nil {
		return &errors.ValidationError{Msg: "invalid email address"}
	}
	return nil
}
