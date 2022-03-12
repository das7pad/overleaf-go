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

package user

import (
	"time"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type AlphaProgramField struct {
	AlphaProgram bool `json:"alphaProgram" edgedb:"alphaProgram"`
}

type AuditLogField struct {
	AuditLog []AuditLogEntry `edgedb:"auditLog"`
}

type BetaProgramField struct {
	BetaProgram bool `json:"betaProgram" edgedb:"betaProgram"`
}

type EditorConfigField struct {
	EditorConfig EditorConfig `json:"ace" edgedb:"ace"`
}

type EmailField struct {
	Email sharedTypes.Email `json:"email" edgedb:"email"`
}

type EmailsField struct {
	Emails []EmailDetails `json:"emails" edgedb:"emails"`
}

type EpochField struct {
	Epoch int64 `edgedb:"epoch"`
}

type FeaturesField struct {
	Features Features `json:"features" edgedb:"features"`
}

type FirstNameField struct {
	FirstName string `json:"first_name" edgedb:"first_name"`
}

type IdField struct {
	Id edgedb.UUID `json:"_id" edgedb:"id"`
}

type IsAdminField struct {
	IsAdmin bool `json:"isAdmin"`
}

type HashedPasswordField struct {
	HashedPassword string `json:"-" edgedb:"password_hash"`
}

type LastLoggedInField struct {
	LastLoggedIn *time.Time `edgedb:"lastLoggedIn"`
}

type LastLoginIpField struct {
	LastLoginIp string `edgedb:"lastLoginIp"`
}

type LastNameField struct {
	LastName string `json:"last_name" edgedb:"last_name"`
}

type LoginCountField struct {
	LoginCount int64 `edgedb:"loginCount"`
}

type MustReconfirmField struct {
	MustReconfirm bool `edgedb:"must_reconfirm"`
}

type SignUpDateField struct {
	SignUpDate time.Time `edgedb:"signUpDate"`
}
