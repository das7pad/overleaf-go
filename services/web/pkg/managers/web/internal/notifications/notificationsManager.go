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

package notifications

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/das7pad/overleaf-go/pkg/models/notification"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	GetUserNotifications(ctx context.Context, request *types.GetNotificationsRequest, response *types.GetNotificationsResponse) error
	RemoveNotification(ctx context.Context, request *types.RemoveNotificationRequest) error
}

func New(db *pgxpool.Pool) Manager {
	return &manager{
		nm: notification.New(db),
	}
}

type manager struct {
	nm notification.Manager
}

func (m *manager) GetUserNotifications(ctx context.Context, request *types.GetNotificationsRequest, response *types.GetNotificationsResponse) error {
	return m.nm.GetAllForUser(ctx, request.Session.User.Id, response)
}

func (m *manager) RemoveNotification(ctx context.Context, r *types.RemoveNotificationRequest) error {
	return m.nm.RemoveById(ctx, r.Session.User.Id, r.NotificationId)
}
