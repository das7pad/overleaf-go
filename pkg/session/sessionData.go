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

package session

import (
	"github.com/das7pad/overleaf-go/pkg/models/oneTimeToken"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

var anonymousUser = &User{}

type User struct {
	Email     sharedTypes.Email `json:"e"`
	FirstName string            `json:"f,omitempty"`
	Id        sharedTypes.UUID  `json:"i"`
	LastName  string            `json:"l,omitempty"`
}

func (u *User) ToPublicUserInfo() user.WithPublicInfo {
	return user.WithPublicInfo{
		EmailField:     user.EmailField{Email: u.Email},
		FirstNameField: user.FirstNameField{FirstName: u.FirstName},
		IdField:        user.IdField{Id: u.Id},
		LastNameField:  user.LastNameField{LastName: u.LastName},
	}
}

type PublicData struct {
	User     *User  `json:"u,omitempty"`
	Language string `json:"l,omitempty"`
}

type anonTokenAccess map[string]project.AccessToken

type Data struct {
	AnonTokenAccess    anonTokenAccess           `json:"ata,omitempty"`
	PasswordResetToken oneTimeToken.OneTimeToken `json:"rt,omitempty"`
	PostLoginRedirect  string                    `json:"plr,omitempty"`
	LoginMetadata      *LoginMetadata            `json:"lm,omitempty"`
	PublicData
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
	if len(d.Language) != 0 {
		return false
	}
	return true
}

func (d *Data) GetAnonTokenAccess(projectId sharedTypes.UUID) project.AccessToken {
	if !d.User.Id.IsZero() {
		// Tokens are cleared during login. Explicitly return no token here.
		return ""
	}
	return d.AnonTokenAccess[projectId.String()]
}

func (d *Data) AddAnonTokenAccess(projectId sharedTypes.UUID, token project.AccessToken) {
	if d.AnonTokenAccess == nil {
		d.AnonTokenAccess = make(anonTokenAccess, 1)
	}
	d.AnonTokenAccess[projectId.String()] = token
}
