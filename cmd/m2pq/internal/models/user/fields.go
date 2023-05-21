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

package user

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type AuditLogField struct {
	AuditLog []AuditLogEntry `bson:"auditLog"`
}

type BetaProgramField struct {
	BetaProgram bool `bson:"betaProgram"`
}

type EditorConfigField struct {
	EditorConfig EditorConfig `bson:"ace"`
}

type EmailField struct {
	Email sharedTypes.Email `bson:"email"`
}

type EmailsField struct {
	Emails []EmailDetails `bson:"emails"`
}

type EpochField struct {
	Epoch int64 `bson:"epoch"`
}

type FeaturesField struct {
	Features Features `bson:"features"`
}

type FirstNameField struct {
	FirstName string `bson:"first_name"`
}

type IdField struct {
	Id primitive.ObjectID `bson:"_id"`
}

type HashedPasswordField struct {
	HashedPassword string `bson:"hashedPassword"`
}

type LastLoggedInField struct {
	LastLoggedIn *time.Time `bson:"lastLoggedIn"`
}

type LastLoginIPField struct {
	LastLoginIP string `bson:"lastLoginIp"`
}

type LastNameField struct {
	LastName string `bson:"last_name"`
}

type LoginCountField struct {
	LoginCount int64 `bson:"loginCount"`
}

type MustReconfirmField struct {
	MustReconfirm bool `bson:"must_reconfirm"`
}

type SignUpDateField struct {
	SignUpDate time.Time `bson:"signUpDate"`
}
