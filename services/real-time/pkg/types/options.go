// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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

type GracefulShutdownOptions struct {
	Delay          time.Duration `json:"delay"`
	Timeout        time.Duration `json:"timeout"`
	CleanupTimeout time.Duration `json:"cleanup_timeout"`
}

func (o *GracefulShutdownOptions) Validate() error {
	if o.Timeout <= 0 {
		return &errors.ValidationError{Msg: "timeout must be greater than 0"}
	}
	if o.CleanupTimeout <= 0 {
		return &errors.ValidationError{
			Msg: "cleanup_timeout must be greater than 0",
		}
	}
	return nil
}

type Options struct {
	GracefulShutdown GracefulShutdownOptions `json:"graceful_shutdown"`

	WriteQueueDepth int `json:"write_queue_depth"`
	BootstrapWorker int `json:"bootstrap_worker"`
	WriteWorker     int `json:"write_worker"`

	JWT JWTOptions `json:"jwt"`
}

type JWTOptions struct {
	Project jwtOptions.JWTOptions `json:"project"`
}

func (o *JWTOptions) Validate() error {
	if err := o.Project.Validate(); err != nil {
		return errors.Tag(err, "project")
	}
	return nil
}

func (o *Options) FillFromEnv() {
	env.MustParseJSON(o, "REAL_TIME_OPTIONS")
	o.JWT.Project.FillFromEnv("JWT_WEB_VERIFY_SECRET")
}

func (o *Options) Validate() error {
	if err := o.GracefulShutdown.Validate(); err != nil {
		return errors.Tag(err, "graceful_shutdown")
	}
	if o.WriteQueueDepth <= 0 {
		return errors.New("write_queue_depth must be greater than 0")
	}
	if err := o.JWT.Validate(); err != nil {
		return errors.Tag(err, "jwt")
	}
	return nil
}
