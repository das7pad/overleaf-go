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

package jwtOptions

import (
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/options/env"
)

type JWTOptions struct {
	Algorithm string        `json:"algo"`
	Key       interface{}   `json:"key"`
	ExpiresIn time.Duration `json:"expires_in"`
}

func (j *JWTOptions) Validate() error {
	if j.Algorithm == "" {
		return &errors.ValidationError{Msg: "missing algo"}
	}
	if j.Key == nil {
		return &errors.ValidationError{Msg: "missing key"}
	}
	if j.ExpiresIn == 0 {
		return &errors.ValidationError{Msg: "missing expires_in"}
	}
	return nil
}

func (j *JWTOptions) FillFromEnv(name string) {
	if j.Algorithm != "" || j.Key != nil {
		return
	}
	j.Algorithm = "HS512"
	j.Key = []byte(env.MustGetString(name))
}

func Parse(key string) JWTOptions {
	j := &JWTOptions{}
	j.FillFromEnv(key)
	return *j
}
