// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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

package editor

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/das7pad/overleaf-go/pkg/jwt/loggedInUserJWT"
	"github.com/das7pad/overleaf-go/pkg/jwt/projectJWT"
	"github.com/das7pad/overleaf-go/pkg/models/message"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/pubSub/channel"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/pkg/templates"
	"github.com/das7pad/overleaf-go/services/filestore/pkg/managers/filestore"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/compile"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/systemMessage"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	GetProjectFile(ctx context.Context, request *types.GetProjectFileRequest, response *types.GetProjectFileResponse) error
	GetProjectFileSize(ctx context.Context, request *types.GetProjectFileSizeRequest, response *types.GetProjectFileSizeResponse) error
	GetUserContacts(ctx context.Context, request *types.GetUserContactsRequest, response *types.GetUserContactsResponse) error
	ListProjectMembers(ctx context.Context, request *types.ListProjectMembersRequest, response *types.ListProjectMembersResponse) error
	LeaveProject(ctx context.Context, request *types.LeaveProjectRequest) error
	RemoveMemberFromProject(ctx context.Context, request *types.RemoveProjectMemberRequest) error
	SetMemberPrivilegeLevelInProject(ctx context.Context, request *types.SetMemberPrivilegeLevelInProjectRequest) error
	TransferProjectOwnership(ctx context.Context, request *types.TransferProjectOwnershipRequest) error
	ProjectEditorPage(ctx context.Context, request *types.ProjectEditorPageRequest, response *types.ProjectEditorPageResponse) error
	ProjectEditorDetached(ctx context.Context, request *types.ProjectEditorDetachedPageRequest, res *types.ProjectEditorDetachedPageResponse) error
	GetProjectJWT(ctx context.Context, request *types.GetProjectJWTRequest, response *types.GetProjectJWTResponse) error
	GetProjectMessages(ctx context.Context, request *types.GetProjectChatMessagesRequest, response *types.GetProjectChatMessagesResponse) error
	SendProjectMessage(ctx context.Context, request *types.SendProjectChatMessageRequest) error
	SetCompiler(ctx context.Context, request *types.SetCompilerRequest) error
	SetImageName(ctx context.Context, request *types.SetImageNameRequest) error
	SetSpellCheckLanguage(ctx context.Context, request *types.SetSpellCheckLanguageRequest) error
	SetRootDocId(ctx context.Context, request *types.SetRootDocIdRequest) error
	GetAccessTokens(ctx context.Context, r *types.GetAccessTokensRequest, response *types.GetAccessTokensResponse) error
	SetPublicAccessLevel(ctx context.Context, request *types.SetPublicAccessLevelRequest, response *types.SetPublicAccessLevelResponse) error
	UpdateEditorConfig(ctx context.Context, request *types.UpdateEditorConfigRequest) error
}

func New(options *types.Options, ps *templates.PublicSettings, client redis.UniversalClient, editorEvents channel.Writer, pm project.Manager, um user.Manager, mm message.Manager, fm filestore.Manager, projectJWTHandler projectJWT.JWTHandler, loggedInUserJWTHandler loggedInUserJWT.JWTHandler, cm compile.Manager, smm systemMessage.Manager) Manager {
	frontendAllowedImageNames := make([]templates.AllowedImageName, 0)
	for _, allowedImageName := range options.AllowedImageNames {
		if !allowedImageName.AdminOnly {
			frontendAllowedImageNames = append(frontendAllowedImageNames, allowedImageName)
		}
	}
	return &manager{
		client:          client,
		cm:              cm,
		mm:              mm,
		editorEvents:    editorEvents,
		fm:              fm,
		jwtProject:      projectJWTHandler,
		jwtLoggedInUser: loggedInUserJWTHandler,
		pm:              pm,
		smm:             smm,
		um:              um,

		adminEmail:                options.AdminEmail,
		appName:                   options.AppName,
		allowedImageNames:         options.AllowedImages,
		emailOptions:              options.EmailOptions(),
		frontendAllowedImageNames: frontendAllowedImageNames,
		ps:                        ps,
		siteURL:                   options.SiteURL,
		smokeTestUserId:           options.SmokeTest.UserId,
	}
}

type manager struct {
	client          redis.UniversalClient
	cm              compile.Manager
	mm              message.Manager
	editorEvents    channel.Writer
	fm              filestore.Manager
	jwtProject      projectJWT.JWTHandler
	jwtLoggedInUser loggedInUserJWT.JWTHandler
	pm              project.Manager
	smm             systemMessage.Manager
	um              user.Manager

	adminEmail                sharedTypes.Email
	appName                   string
	allowedImageNames         []sharedTypes.ImageName
	emailOptions              *types.EmailOptions
	frontendAllowedImageNames []templates.AllowedImageName
	ps                        *templates.PublicSettings
	siteURL                   sharedTypes.URL
	smokeTestUserId           sharedTypes.UUID
}

func (m *manager) notifyEditor(projectId sharedTypes.UUID, message sharedTypes.EditorEventMessage, payload interface{}) {
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	blob, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = m.editorEvents.Publish(ctx, &sharedTypes.EditorEvent{
		RoomId:  projectId,
		Message: message,
		Payload: blob,
	})
}

type refreshMembershipDetails struct {
	Invites bool             `json:"invites,omitempty"`
	Members bool             `json:"members,omitempty"`
	Owner   bool             `json:"owner,omitempty"`
	UserId  sharedTypes.UUID `json:"userId"`
}

func (m *manager) notifyEditorAboutAccessChanges(projectId sharedTypes.UUID, r refreshMembershipDetails) {
	m.notifyEditor(projectId, sharedTypes.ProjectMembershipChanged, r)
}
