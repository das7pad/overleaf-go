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

package login

import (
	"context"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/session"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/pkg/templates"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) bumpEpoch(ctx context.Context, userId sharedTypes.UUID) error {
	if err := m.um.BumpEpoch(ctx, userId); err != nil {
		return errors.Tag(err, "cannot bump user epoch in db")
	}
	return nil
}

type clearSessionsAuditLogInfo struct {
	Sessions []session.OtherSessionData `json:"sessions"`
}

func (m *manager) ClearSessions(ctx context.Context, request *types.ClearSessionsRequest) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	userId := request.Session.User.Id
	u := &user.WithPublicInfo{}
	if err := m.um.GetUser(ctx, userId, u); err != nil {
		return errors.Tag(err, "cannot get user")
	}

	// Do the actual cleanup and keep track of it in the audit log.
	var errDestroy error
	for i := 0; i < 3; i++ {
		errDestroy = m.destroySessionsOnce(ctx, request)
		if errDestroy == context.DeadlineExceeded && ctx.Err() == nil {
			continue
		}
		break
	}

	// Confirm cleanup with another bump of the user epoch.
	errBump := m.bumpEpoch(ctx, userId)
	if errDestroy == nil && errBump != nil {
		return errors.Tag(errBump, "cannot bump epoch after of destroying")
	}
	if errDestroy != nil {
		return errors.Tag(errDestroy, "cannot destroy other sessions")
	}

	return m.emailSecurityAlert(
		ctx,
		u,
		"active sessions cleared",
		"active sessions were cleared on your account "+string(u.Email),
	)
}

var errNothingToClear = &errors.InvalidStateError{Msg: "nothing to clear"}

func (m *manager) destroySessionsOnce(ctx context.Context, request *types.ClearSessionsRequest) error {
	// Do not process stale redis data.
	ctx, done := context.WithTimeout(ctx, 10*time.Second)
	defer done()

	userId := request.Session.User.Id
	d, err := request.Session.GetOthers(ctx)
	if err != nil {
		return errors.Tag(err, "cannot get other sessions")
	}
	if len(d.Sessions) == 0 {
		return errNothingToClear
	}
	info := &clearSessionsAuditLogInfo{Sessions: d.Sessions}

	// Add audit log entry and bump user epoch.
	err = m.um.TrackClearSessions(ctx, userId, request.IPAddress, info)
	if err != nil {
		return errors.Tag(err, "cannot track session clearing in db")
	}

	if deadline, ok := ctx.Deadline(); ok {
		if deadline.Sub(time.Now()) < time.Second {
			// Chicken out when we are close to exceeding the deadline.
			return context.DeadlineExceeded
		}
	}

	// Do the actual cleanup
	if err = request.Session.DestroyOthers(ctx, d); err != nil {
		return errors.Tag(err, "cannot destroy other sessions")
	}
	return nil
}

func (m *manager) SessionsPage(ctx context.Context, request *types.SessionsPageRequest, response *types.SessionsPageResponse) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}

	otherSessions, err := request.Session.GetOthers(ctx)
	if err != nil {
		return err
	}

	response.Data = &templates.UserSessionsData{
		MarketingLayoutData: templates.MarketingLayoutData{
			CommonData: templates.CommonData{
				Settings:    m.ps,
				SessionUser: request.Session.User,
				TitleLocale: "sessions",
			},
		},
		CurrentSession: request.Session.ToOtherSessionData(),
		OtherSessions:  otherSessions.Sessions,
	}
	return nil
}
