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

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type EmailDetails struct {
	Id               primitive.ObjectID           `bson:"_id"`
	CreatedAt        time.Time                    `json:"createdAt" bson:"createdAt"`
	ConfirmedAt      *time.Time                   `json:"confirmedAt" bson:"confirmedAt"`
	ReconfirmedAt    *time.Time                   `json:"reconfirmedAt" bson:"reconfirmedAt"`
	Email            sharedTypes.Email            `json:"email" bson:"email"`
	ReversedHostname sharedTypes.ReversedHostname `json:"reversedHostname" bson:"reversedHostname"`
}
