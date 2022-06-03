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

package user

import (
	"time"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type EmailDetails struct {
	ConfirmedAt *time.Time        `json:"confirmedAt" edgedb:"confirmed_at"`
	Email       sharedTypes.Email `json:"email" edgedb:"email"`
}

type EmailDetailsWithDefaultFlag struct {
	EmailDetails
	IsDefault bool `json:"default"`
}

func (u *ProjectListViewCaller) ToUserEmails() []EmailDetailsWithDefaultFlag {
	return []EmailDetailsWithDefaultFlag{
		{
			EmailDetails: EmailDetails{
				ConfirmedAt: u.EmailConfirmedAt,
				Email:       u.Email,
			},
			IsDefault: true,
		},
	}
}
