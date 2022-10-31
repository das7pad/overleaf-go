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

package signedCookie

import (
	"strings"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/options/utils"
)

type Options struct {
	Domain  string        `json:"domain"`
	Expiry  time.Duration `json:"expiry"`
	Name    string        `json:"name"`
	Path    string        `json:"path"`
	Secrets []string      `json:"secrets"`
	Secure  bool          `json:"secure"`
}

func (o *Options) FillFromEnv(name string) {
	if len(o.Secrets) > 0 {
		return
	}
	o.Secrets = strings.Split(utils.MustGetStringFromEnv(name), ",")
}

func (o *Options) Validate() error {
	if o.Domain == "" {
		return &errors.ValidationError{Msg: "missing domain"}
	}
	if o.Expiry == 0 {
		return &errors.ValidationError{Msg: "missing expiry"}
	}
	if o.Name == "" {
		return &errors.ValidationError{Msg: "missing name"}
	}
	if o.Path == "" {
		return &errors.ValidationError{Msg: "missing path"}
	}
	if len(o.Secrets) == 0 {
		return &errors.ValidationError{Msg: "missing secrets"}
	}
	return nil
}
