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
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/das7pad/overleaf-go/pkg/email"
	"github.com/das7pad/overleaf-go/pkg/email/pkg/gmailGoToAction"
	"github.com/das7pad/overleaf-go/pkg/email/pkg/spamSafe"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/notification"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/projectInvite"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/pubSub/channel"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/pkg/templates"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	AcceptProjectInvite(ctx context.Context, request *types.AcceptProjectInviteRequest, response *types.AcceptProjectInviteResponse) error
	CreateProjectInvite(ctx context.Context, request *types.CreateProjectInviteRequest) error
	ListProjectInvites(ctx context.Context, request *types.ListProjectInvitesRequest, response *types.ListProjectInvitesResponse) error
	ResendProjectInvite(ctx context.Context, request *types.ResendProjectInviteRequest) error
	RevokeProjectInvite(ctx context.Context, request *types.RevokeProjectInviteRequest) error
	ViewProjectInvite(ctx context.Context, request *types.ViewProjectInvitePageRequest, response *types.ViewProjectInvitePageResponse) error
}

func New(options *types.Options, ps *templates.PublicSettings, db *pgxpool.Pool, editorEvents channel.Writer, pm project.Manager, um user.Manager) Manager {
	return &manager{
		editorEvents: editorEvents,
		nm:           notification.New(db),
		pim:          projectInvite.New(db),
		pm:           pm,
		um:           um,

		appName:      options.AppName,
		emailOptions: options.EmailOptions(),
		ps:           ps,
		siteURL:      options.SiteURL,
	}
}

type manager struct {
	editorEvents channel.Writer
	nm           notification.Manager
	pim          projectInvite.Manager
	pm           project.Manager
	um           user.Manager

	appName      string
	emailOptions *types.EmailOptions
	ps           *templates.PublicSettings
	siteURL      sharedTypes.URL
}

func getKey(inviteId sharedTypes.UUID) string {
	return "project-invite-" + inviteId.String()
}

type refreshMembershipDetails struct {
	Invites bool `json:"invites,omitempty"`
	Members bool `json:"members,omitempty"`
}

func (m *manager) notifyEditorAboutChanges(projectId sharedTypes.UUID, r refreshMembershipDetails) {
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()

	payload := []interface{}{r}
	if b, err2 := json.Marshal(payload); err2 == nil {
		_ = m.editorEvents.Publish(ctx, &sharedTypes.EditorEventsMessage{
			RoomId:  projectId,
			Message: "project:membership:changed",
			Payload: b,
		})
	}
}

func (m *manager) resendNotification(ctx context.Context, d *projectInviteDetails) error {
	if !d.IsUserRegistered() {
		return nil
	}

	n := notification.Notification{}
	n.Expires = d.invite.Expires
	n.Key = getKey(d.invite.Id)
	n.TemplateKey = "notification_project_invite"
	n.UserId = d.user.Id
	{
		blob, err := json.Marshal(map[string]interface{}{
			"userName":    d.sender.DisplayName(),
			"projectName": d.project.Name,
			"projectId":   d.invite.ProjectId.String(),
			"token":       d.invite.Token,
		})
		if err != nil {
			return errors.Tag(err, "cannot serialize notification options")
		}
		n.MessageOptions = blob
	}
	if err := m.nm.Resend(ctx, n); err != nil {
		return errors.Tag(err, "cannot create invite notification")
	}
	return nil
}

func (m *manager) sendEmail(ctx context.Context, d *projectInviteDetails) error {
	inviteURL := d.GetInviteURL(m.siteURL)
	p := d.project
	s := d.sender
	u := d.user

	message := fmt.Sprintf(
		"%s wants to share %s with you.",
		spamSafe.GetSafeEmail(s.Email, "a collaborator"),
		spamSafe.GetSafeProjectName(p.Name, "a new project"),
	)
	title := fmt.Sprintf(
		"%s - shared by %s",
		spamSafe.GetSafeProjectName(p.Name, "New Project"),
		spamSafe.GetSafeEmail(s.Email, "a collaborator"),
	)

	e := email.Email{
		Content: &email.CTAContent{
			PublicOptions: m.emailOptions.Public,
			Message:       email.Message{message},
			Title:         title,
			CTAText:       "View project",
			CTAURL:        inviteURL,
			GmailGoToAction: &gmailGoToAction.GmailGoToAction{
				Target: inviteURL,
				Name:   "View project",
				Description: fmt.Sprintf(
					"Join %s at %s",
					spamSafe.GetSafeProjectName(p.Name, "project"),
					m.appName,
				),
			},
		},
		ReplyTo: email.Identity{
			Address: s.Email,
			DisplayName: spamSafe.GetSafeUserName(
				s.DisplayName(), "Project owner",
			),
		},
		Subject: title,
		To: email.Identity{
			Address: u.Email,
			DisplayName: spamSafe.GetSafeUserName(
				u.DisplayName(), "",
			),
		},
	}
	if err := e.Send(ctx, m.emailOptions.Send); err != nil {
		return errors.Tag(err, "cannot send email")
	}
	return nil
}
