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

	"github.com/das7pad/overleaf-go/pkg/models/oneTimeToken"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/pkg/templates"
)

type ConfirmEmailRequest struct {
	Token oneTimeToken.OneTimeToken `json:"token"`
}

func (r *ConfirmEmailRequest) Validate() error {
	if err := r.Token.Validate(); err != nil {
		return err
	}
	return nil
}

type ConfirmEmailPageRequest struct {
	WithSession
	Token oneTimeToken.OneTimeToken `form:"token"`
}

func (r *ConfirmEmailPageRequest) FromQuery(q url.Values) error {
	r.Token = oneTimeToken.OneTimeToken(q.Get("token"))
	return nil
}

func (r *ConfirmEmailPageRequest) Validate() error {
	if err := r.Token.Validate(); err != nil {
		return err
	}
	return nil
}

type ConfirmEmailPageResponse struct {
	Data *templates.UserConfirmEmailData
}

type ResendEmailConfirmationRequest struct {
	WithSession

	Email sharedTypes.Email `json:"email"`
}

func (r *ResendEmailConfirmationRequest) Preprocess() {
	r.Email = r.Email.Normalize()
}

func (r *ResendEmailConfirmationRequest) Validate() error {
	if err := r.Email.Validate(); err != nil {
		return err
	}
	return nil
}

type ReconfirmAccountPageRequest struct {
	WithSession
}

type ReconfirmAccountPageResponse struct {
	Data     *templates.UserReconfirmData
	Redirect string
}
