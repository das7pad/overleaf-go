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

package types

import (
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/objectStorage"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Buckets struct {
	UserFiles string `json:"user_files"`
}

type Options struct {
	AllowRedirects bool                  `json:"allow_redirects"`
	BackendOptions objectStorage.Options `json:"backend_options"`
	Buckets        Buckets               `json:"buckets"`
	UploadBase     sharedTypes.DirName   `json:"upload_base"`
}

func (o *Options) Validate() error {
	if o.Buckets.UserFiles == "" {
		return &errors.ValidationError{
			Msg: "missing buckets.user_files",
		}
	}
	if len(o.UploadBase) == 0 {
		return &errors.ValidationError{Msg: "missing upload_base"}
	}
	return nil
}
