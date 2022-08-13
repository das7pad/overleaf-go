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

package session

import (
	"time"

	"github.com/das7pad/overleaf-go/pkg/models/oneTimeToken"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

var anonymousUser = &User{
	Epoch: user.AnonymousUserEpoch,
}

//goland:noinspection SpellCheckingInspection
type User struct {
	Email          sharedTypes.Email `json:"email"`
	Epoch          int64             `json:"epoch,omitempty"`
	FirstName      string            `json:"first_name,omitempty"`
	IPAddress      string            `json:"ip_address"`
	Id             sharedTypes.UUID  `json:"_id,omitempty"`
	IsAdmin        bool              `json:"isAdmin,omitempty"`
	LastName       string            `json:"last_name,omitempty"`
	Language       string            `json:"lng"`
	SessionCreated time.Time         `json:"session_created"`
}

func (u *User) ToPublicUserInfo() user.WithPublicInfo {
	return user.WithPublicInfo{
		EmailField:     user.EmailField{Email: u.Email},
		FirstNameField: user.FirstNameField{FirstName: u.FirstName},
		IdField:        user.IdField{Id: u.Id},
		LastNameField:  user.LastNameField{LastName: u.LastName},
	}
}

type anonTokenAccess map[string]project.AccessToken

type Data struct {
	AnonTokenAccess    anonTokenAccess           `json:"anonTokenAccess,omitempty"`
	PasswordResetToken oneTimeToken.OneTimeToken `json:"resetToken,omitempty"`
	PostLoginRedirect  string                    `json:"postLoginRedirect,omitempty"`
	User               *User                     `json:"user,omitempty"`
}

func (d *Data) IsEmpty() bool {
	if d == nil {
		return true
	}
	if d.PasswordResetToken != "" {
		return false
	}
	if d.PostLoginRedirect != "" {
		return false
	}
	if d.User != anonymousUser {
		return false
	}
	if len(d.AnonTokenAccess) != 0 {
		return false
	}
	return true
}

func (d *Data) GetAnonTokenAccess(projectId sharedTypes.UUID) project.AccessToken {
	return d.AnonTokenAccess[projectId.String()]
}

func (d *Data) AddAnonTokenAccess(projectId sharedTypes.UUID, token project.AccessToken) {
	if d.AnonTokenAccess == nil {
		d.AnonTokenAccess = make(anonTokenAccess, 1)
	}
	d.AnonTokenAccess[projectId.String()] = token
}
