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

package sharedTypes

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

func ParseAndValidateURL(s string) (*URL, error) {
	raw, err := url.Parse(s)
	if err != nil {
		return nil, &errors.ValidationError{Msg: "invalid url: " + err.Error()}
	}
	if raw.Path == "" {
		raw.Path = "/"
	}
	u := URL{URL: *raw}
	if err = u.Validate(); err != nil {
		return nil, err
	}
	return &u, nil
}

type URL struct {
	url.URL
}

func (u *URL) FileNameFromPath() Filename {
	return PathName(u.Path).Filename()
}

func (u URL) WithPath(s string) *URL {
	if s == "" {
		// no-op
	} else if strings.HasSuffix(u.Path, "/") {
		if strings.HasPrefix(s, "/") {
			u.Path += s[1:]
		} else {
			u.Path += s
		}
	} else {
		if strings.HasPrefix(s, "/") {
			u.Path += s
		} else {
			u.Path += "/" + s
		}
	}
	return &u
}

func (u URL) WithQuery(values url.Values) *URL {
	u.URL.RawQuery = values.Encode()
	return &u
}

func (u *URL) UnmarshalJSON(bytes []byte) error {
	var s string
	if err := json.Unmarshal(bytes, &s); err != nil {
		return err
	}
	v, err := ParseAndValidateURL(s)
	if err != nil {
		return err
	}
	*u = *v
	return nil
}

func (u *URL) MarshalJSON() ([]byte, error) {
	s := u.URL.String()
	return json.Marshal(s)
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
	if u.Path == "" {
		return &errors.ValidationError{
			Msg: "URL is missing path",
		}
	}
	return nil
}
