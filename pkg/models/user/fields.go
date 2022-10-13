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

type AlphaProgramField struct {
	AlphaProgram bool `json:"alphaProgram"`
}

type AuditLogField struct {
	AuditLog []AuditLogEntry
}

type BetaProgramField struct {
	BetaProgram bool `json:"betaProgram"`
}

type EditorConfigField struct {
	EditorConfig EditorConfig `json:"ace"`
}

type EmailField struct {
	Email sharedTypes.Email `json:"email"`
}

type EmailConfirmedAtField struct {
	EmailConfirmedAt *time.Time `json:"emailConfirmedAt"`
}

type EmailsField struct {
	Emails []EmailDetails `json:"emails"`
}

type EpochField struct {
	Epoch int64 `json:"epoch"`
}

type FeaturesField struct {
	Features Features `json:"features"`
}

type FirstNameField struct {
	FirstName string `json:"first_name"`
}

type IdField struct {
	Id sharedTypes.UUID `json:"_id"`
}

type IsAdminField struct {
	IsAdmin bool `json:"isAdmin"`
}

type HashedPasswordField struct {
	HashedPassword string `json:"-"`
}

type LastLoggedInField struct {
	LastLoggedIn *time.Time
}

type LastLoginIpField struct {
	LastLoginIp string
}

type LastNameField struct {
	LastName string `json:"last_name"`
}

type LearnedWordsField struct {
	LearnedWords []string `json:"learnedWords"`
}

type LoginCountField struct {
	LoginCount int64
}

type MustReconfirmField struct {
	MustReconfirm bool
}

type SignUpDateField struct {
	SignUpDate time.Time
}
