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

package session

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

var (
	anonymousUser = &User{
		Epoch: user.AnonymousUserEpoch,
	}
)

type User struct {
	Id             primitive.ObjectID `json:"_id,omitempty"`
	IsAdmin        bool               `json:"isAdmin,omitempty"`
	FirstName      string             `json:"first_name,omitempty"`
	LastName       string             `json:"last_name,omitempty"`
	Email          sharedTypes.Email  `json:"email"`
	Epoch          int64              `json:"epoch,omitempty"`
	ReferralId     string             `json:"referal_id"`
	IPAddress      string             `json:"ip_address"`
	SessionCreated time.Time          `json:"session_created"`
}

type AnonTokenAccess map[string]project.AccessToken

type Data struct {
	AnonTokenAccess   AnonTokenAccess `json:"anonTokenAccess,omitempty"`
	PostLoginRedirect string          `json:"postLoginRedirect,omitempty"`
	User              *User           `json:"user,omitempty"`
}
