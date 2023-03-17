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

package types

import (
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/options/env"
	"github.com/das7pad/overleaf-go/pkg/options/jwtOptions"
)

type Options struct {
	GracefulShutdown struct {
		Delay   time.Duration `json:"delay"`
		Timeout time.Duration `json:"timeout"`
	} `json:"graceful_shutdown"`

	JWT struct {
		Project jwtOptions.JWTOptions `json:"project"`
	} `json:"jwt"`
}

func (o *Options) FillFromEnv() {
	env.MustParseJSON(o, "REAL_TIME_OPTIONS")
	o.JWT.Project.FillFromEnv("JWT_WEB_VERIFY_SECRET")
}

func (o *Options) Validate() error {
	if o.GracefulShutdown.Timeout <= 0 {
		return &errors.ValidationError{
			Msg: "graceful_shutdown.timeout must be greater than 0",
		}
	}
	return nil
}
