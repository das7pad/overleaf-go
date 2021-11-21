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

package projectInvite

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/pkg/email"
	"github.com/das7pad/overleaf-go/pkg/email/pkg/gmailGoToAction"
	"github.com/das7pad/overleaf-go/pkg/email/pkg/spamSafe"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/projectInvite"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/pubSub/channel"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/contacts/pkg/managers/contacts"
	"github.com/das7pad/overleaf-go/services/notifications/pkg/managers/notifications"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	AcceptProjectInvite(ctx context.Context, request *types.AcceptProjectInviteRequest, response *types.AcceptProjectInviteResponse) error
	CreateProjectInvite(ctx context.Context, request *types.CreateProjectInviteRequest) error
	ListProjectInvites(ctx context.Context, request *types.ListProjectInvitesRequest, response *types.ListProjectInvitesResponse) error
	ResendProjectInvite(ctx context.Context, request *types.ResendProjectInviteRequest) error
	RevokeProjectInvite(ctx context.Context, request *types.RevokeProjectInviteRequest) error
}

func New(options *types.Options, client redis.UniversalClient, db *mongo.Database, editorEvents channel.Writer, pm project.Manager, um user.Manager, cm contacts.Manager, nm notifications.Manager) Manager {
	return &manager{
		client:       client,
		cm:           cm,
		db:           db,
		editorEvents: editorEvents,
		emailOptions: options.EmailOptions(),
		nm:           nm,
		options:      options,
		pim:          projectInvite.New(db),
		pm:           pm,
		um:           um,
	}
}

type manager struct {
	client       redis.UniversalClient
	cm           contacts.Manager
	db           *mongo.Database
	editorEvents channel.Writer
	emailOptions *types.EmailOptions
	nm           notifications.Manager
	options      *types.Options
	pim          projectInvite.Manager
	pm           project.Manager
	um           user.Manager
}

type refreshMembershipDetails struct {
	Invites bool `json:"invites,omitempty"`
	Members bool `json:"members,omitempty"`
}

func (m *manager) notifyEditorAboutChanges(projectId primitive.ObjectID, r *refreshMembershipDetails) {
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

func (m *manager) createNotification(ctx context.Context, d *projectInviteDetails) error {
	if !d.IsUserRegistered() {
		return nil
	}

	key := "project-invite-" + d.invite.Id.Hex()
	userId := d.user.Id

	opts := &bson.M{
		"userName":    d.sender.DisplayName(),
		"projectName": d.project.Name,
		"projectId":   d.invite.ProjectId.Hex(),
		"token":       d.invite.Token,
	}
	n := notifications.Notification{
		Key:            key,
		UserId:         userId,
		Expires:        d.invite.Expires,
		TemplateKey:    "notification_project_invite",
		MessageOptions: opts,
	}
	if err := m.nm.AddNotification(ctx, userId, n, true); err != nil {
		return errors.Tag(err, "cannot create invite notification")
	}
	return nil
}

func (m *manager) sendEmail(ctx context.Context, d *projectInviteDetails) error {
	inviteURL := d.GetInviteURL(m.options.SiteURL)
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
					m.options.AppName,
				),
			},
		},
		ReplyTo: &email.Identity{
			Address:   s.Email,
			FirstName: spamSafe.GetSafeUserName(s.FirstName, ""),
			DisplayName: spamSafe.GetSafeUserName(
				s.DisplayName(), "Project owner",
			),
		},
		Subject: title,
		To: &email.Identity{
			Address:   u.Email,
			FirstName: spamSafe.GetSafeUserName(u.FirstName, ""),
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
