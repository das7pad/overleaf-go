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

package deletedUser

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type DeleterDataDeletedUserIdField struct {
	DeletedUserId primitive.ObjectID `bson:"deletedUserId"`
}

type DeleterData struct {
	DeleterDataDeletedUserIdField `bson:"inline"`
	DeletedAt                     time.Time          `bson:"deletedAt"`
	DeleterId                     primitive.ObjectID `bson:"deleterId"`
	DeleterIpAddress              string             `bson:"deleterIpAddress"`
	DeletedUserLastLoggedIn       time.Time          `bson:"deletedUserLastLoggedIn"`
	DeletedUserSignUpDate         time.Time          `bson:"deletedUserSignUpDate"`
	DeletedUserLoginCount         int64              `bson:"deletedUserLoginCount"`
	DeletedUserReferralId         string             `bson:"deletedUserReferralId"`
}

type deleterDataOnlyUserId struct {
	DeleterDataDeletedUserIdField `bson:"inline"`
}

type forListing struct {
	DeleterData deleterDataOnlyUserId `bson:"deleterData"`
}
