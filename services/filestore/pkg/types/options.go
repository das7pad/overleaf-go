// Golang port of the Overleaf filestore service
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

package types

import (
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/filestore/pkg/backend"
)

type Buckets struct {
	UserFiles string `json:"user_files"`
}

type Options struct {
	AllowRedirects bool            `json:"allow_redirects"`
	BackendOptions backend.Options `json:"backend_options"`
	Buckets        Buckets         `json:"buckets"`
}

func (o *Options) Validate() error {
	if o.Buckets.UserFiles == "" {
		return &errors.ValidationError{
			Msg: "missing buckets.user_files",
		}
	}
	return nil
}
