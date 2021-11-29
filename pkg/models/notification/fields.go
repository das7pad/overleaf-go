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

package notification

import (
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type IdField struct {
	Id primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
}

type KeyField struct {
	Key string `json:"key" bson:"key"`
}

type UserIdField struct {
	UserId primitive.ObjectID `json:"user_id" bson:"user_id"`
}

type ExpiresField struct {
	Expires time.Time `json:"expires,omitempty" bson:"expires,omitempty"`
}

type TemplateKeyField struct {
	TemplateKey string `json:"templateKey,omitempty" bson:"templateKey,omitempty"`
}

type MessageOptsField struct {
	MessageOptions *bson.M `json:"messageOpts,omitempty" bson:"messageOpts,omitempty"`
}