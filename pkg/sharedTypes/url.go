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
	"encoding/json"
	"net/url"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type URL struct {
	url.URL
}

func (u URL) WithPath(s string) URL {
	u.URL.Path = s
	return u
}

func (u *URL) UnmarshalJSON(bytes []byte) error {
	var s string
	if err := json.Unmarshal(bytes, &s); err != nil {
		return err
	}
	raw, err := url.Parse(s)
	if err != nil {
		return err
	}
	u.URL = *raw
	return nil
}

func (u *URL) MarshalJSON() ([]byte, error) {
	s := u.URL.String()
	return json.Marshal(&s)
}

func (u *URL) Validate() error {
	if u.Scheme == "" {
		return &errors.ValidationError{
			Msg: "URL is missing scheme",
		}
	}
	if u.Host == "" {
		return &errors.ValidationError{
			Msg: "URL is missing host",
		}
	}
	return nil
}
