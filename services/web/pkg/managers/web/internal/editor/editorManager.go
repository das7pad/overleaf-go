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

package editor

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/jwt/jwtHandler"
	"github.com/das7pad/overleaf-go/pkg/jwt/userIdJWT"
	"github.com/das7pad/overleaf-go/pkg/jwt/wsBootstrap"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/pubSub/channel"
	"github.com/das7pad/overleaf-go/services/chat/pkg/managers/chat"
	"github.com/das7pad/overleaf-go/services/contacts/pkg/managers/contacts"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/managers/docstore"
	"github.com/das7pad/overleaf-go/services/filestore/pkg/managers/filestore"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	GetProjectFile(ctx context.Context, request *types.GetProjectFileRequest, response *types.GetProjectFileResponse) error
	GetProjectFileSize(ctx context.Context, request *types.GetProjectFileSizeRequest, response *types.GetProjectFileSizeResponse) error
	GetUserContacts(ctx context.Context, request *types.GetUserContactsRequest, response *types.GetUserContactsResponse) error
	LoadEditor(ctx context.Context, request *types.LoadEditorRequest, response *types.LoadEditorResponse) error
	GetProjectJWT(ctx context.Context, request *types.GetProjectJWTRequest, response *types.GetProjectJWTResponse) error
	GetProjectMessages(ctx context.Context, request *types.GetProjectChatMessagesRequest, response *types.GetProjectChatMessagesResponse) error
	GetWSBootstrap(ctx context.Context, request *types.GetWSBootstrapRequest, response *types.GetWSBootstrapResponse) error
	SendProjectMessage(ctx context.Context, request *types.SendProjectChatMessageRequest) error
}

func New(options *types.Options, editorEvents channel.Writer, pm project.Manager, um user.Manager, cm chat.Manager, csm contacts.Manager, dm docstore.Manager, fm filestore.Manager, projectJWTHandler jwtHandler.JWTHandler, loggedInUserJWTHandler jwtHandler.JWTHandler) Manager {
	return &manager{
		cm:              cm,
		csm:             csm,
		dm:              dm,
		editorEvents:    editorEvents,
		fm:              fm,
		jwtProject:      projectJWTHandler,
		jwtLoggedInUser: loggedInUserJWTHandler,
		jwtSpelling:     userIdJWT.New(options.JWT.Spelling),
		pm:              pm,
		um:              um,
		wsBootstrap:     wsBootstrap.New(options.JWT.RealTime),
	}
}

type manager struct {
	cm              chat.Manager
	csm             contacts.Manager
	dm              docstore.Manager
	editorEvents    channel.Writer
	fm              filestore.Manager
	jwtProject      jwtHandler.JWTHandler
	jwtLoggedInUser jwtHandler.JWTHandler
	jwtSpelling     jwtHandler.JWTHandler
	pm              project.Manager
	um              user.Manager
	wsBootstrap     jwtHandler.JWTHandler
}
