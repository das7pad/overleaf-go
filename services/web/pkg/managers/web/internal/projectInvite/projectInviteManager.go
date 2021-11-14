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
	"time"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/projectInvite"
	"github.com/das7pad/overleaf-go/pkg/pubSub/channel"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/contacts/pkg/managers/contacts"
	"github.com/das7pad/overleaf-go/services/notifications/pkg/managers/notifications"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	AcceptProjectInvite(ctx context.Context, request *types.AcceptProjectInviteRequest, response *types.AcceptProjectInviteResponse) error
}

func New(client redis.UniversalClient, db *mongo.Database, editorEvents channel.Writer, pm project.Manager, cm contacts.Manager, nm notifications.Manager) Manager {
	return &manager{
		client:       client,
		cm:           cm,
		db:           db,
		editorEvents: editorEvents,
		nm:           nm,
		pim:          projectInvite.New(db),
		pm:           pm,
	}
}

type manager struct {
	client       redis.UniversalClient
	cm           contacts.Manager
	db           *mongo.Database
	editorEvents channel.Writer
	nm           notifications.Manager
	pim          projectInvite.Manager
	pm           project.Manager
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
