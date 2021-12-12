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

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	AuditLogOperationChangePrimaryEmail = "change-primary-email"
	AuditLogOperationClearSessions      = "clear-sessions"
	AuditLogOperationLogin              = "login"
	AuditLogOperationResetPassword      = "reset-password"
	AuditLogOperationUpdatePassword     = "update-password"
)

type AuditLogEntry struct {
	Info        interface{}        `bson:"info,omitempty"`
	InitiatorId primitive.ObjectID `bson:"initiatorId"`
	IpAddress   string             `bson:"ipAddress"`
	Operation   string             `bson:"operation"`
	Timestamp   time.Time          `bson:"timestamp"`
}
