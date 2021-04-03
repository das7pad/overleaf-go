// Golang port of the Overleaf chat service
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

package types

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Message struct {
	Id        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Content   string             `json:"content" bson:"content"`
	Timestamp float64            `json:"timestamp" bson:"timestamp"`
	UserId    primitive.ObjectID `json:"user_id" bson:"user_id"`
	EditedAt  float64            `json:"edited_at,omitempty" bson:"edited_at,omitempty"`
	RoomId    primitive.ObjectID `json:"room_id,omitempty" bson:"room_id"`
}

type Thread struct {
	Resolved         *bool               `json:"resolved,omitempty"`
	ResolvedAt       *time.Time          `json:"resolved_at,omitempty"`
	ResolvedByUserId *primitive.ObjectID `json:"resolved_by_user_id,omitempty"`
	Messages         []Message           `json:"messages"`
}
