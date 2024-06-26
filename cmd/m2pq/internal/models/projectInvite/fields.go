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

package projectInvite

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type CreatedAtField struct {
	CreatedAt time.Time `bson:"createdAt"`
}

type EmailField struct {
	Email sharedTypes.Email `bson:"email"`
}

type ExpiresAtField struct {
	Expires time.Time `bson:"expires"`
}

type IdField struct {
	Id primitive.ObjectID `bson:"_id"`
}

type PrivilegeLevelField struct {
	PrivilegeLevel sharedTypes.PrivilegeLevel `bson:"privileges"`
}

type ProjectIdField struct {
	ProjectId primitive.ObjectID `bson:"projectId"`
}

type SendingUserIdField struct {
	SendingUserId primitive.ObjectID `bson:"sendingUserId"`
}

type TokenField struct {
	Token Token `bson:"token"`
}
