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

package oneTimeToken

import (
	"time"
)

type UseField struct {
	Use string `bson:"use"`
}

type TokenField struct {
	Token OneTimeToken `bson:"token"`
}

type CreatedAtField struct {
	CreatedAt time.Time `bson:"createdAt"`
}

type ExpiresAtField struct {
	ExpiresAt time.Time `bson:"expiresAt"`
}

type UsedAtField struct {
	UsedAt time.Time `bson:"usedAt"`
}

type PasswordResetDataField struct {
	PasswordResetData PasswordResetData `bson:"data"`
}
