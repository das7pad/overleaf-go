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

package types

import (
	"net/url"
	"os"
	"strings"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/session"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type CreateMultiProjectZIPRequest struct {
	Session    *session.Session   `json:"-"`
	ProjectIds []sharedTypes.UUID `json:"-"`
}

func (r *CreateMultiProjectZIPRequest) FromQuery(q url.Values) error {
	for _, raw := range strings.Split(q.Get("project_ids"), ",") {
		id, err := sharedTypes.ParseUUID(raw)
		if err != nil {
			return &errors.ValidationError{
				Msg: "invalid project id: " + err.Error(),
			}
		}
		r.ProjectIds = append(r.ProjectIds, id)
	}
	return nil
}

func (r *CreateMultiProjectZIPRequest) Validate() error {
	if len(r.ProjectIds) == 0 {
		return &errors.ValidationError{Msg: "must provide at least one project id"}
	}
	return nil
}

type CreateProjectZIPRequest struct {
	Session   *session.Session `json:"-"`
	ProjectId sharedTypes.UUID `json:"-"`
}

type CreateProjectZIPResponse struct {
	Filename sharedTypes.Filename `json:"-"`
	FSPath   string
}

func (r *CreateProjectZIPResponse) Cleanup() {
	if r.FSPath != "" {
		_ = os.Remove(r.FSPath)
	}
}
