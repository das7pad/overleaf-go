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

package web

import (
	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/pkg/jwt/jwtHandler"
	"github.com/das7pad/overleaf-go/pkg/jwt/loggedInUserJWT"
	"github.com/das7pad/overleaf-go/pkg/jwt/projectJWT"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/tag"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/pubSub/channel"
	"github.com/das7pad/overleaf-go/pkg/session"
	"github.com/das7pad/overleaf-go/services/chat/pkg/managers/chat"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/managers/docstore"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/compile"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/editor"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/login"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/projectList"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/projectMetadata"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/systemMessage"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	GetProjectJWTHandler() jwtHandler.JWTHandler
	GetLoggedInUserJWTHandler() jwtHandler.JWTHandler

	compile.Manager
	editor.Manager
	login.Manager
	projectList.Manager
	projectMetadata.Manager
	session.Manager
	systemMessage.Manager
}

func New(options *types.Options, db *mongo.Database, client redis.UniversalClient) (Manager, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	editorEvents := channel.NewWriter(client, "editor-events")
	chatM := chat.New(db)
	dum, err := documentUpdater.New(
		options.APIs.DocumentUpdater.Options, client, db,
	)
	if err != nil {
		return nil, err
	}
	dm, err := docstore.New(options.APIs.Docstore.Options, db)
	if err != nil {
		return nil, err
	}
	pm := project.New(db)
	smm := systemMessage.New(db)
	tm := tag.New(db)
	um := user.New(db)
	cm, err := compile.New(options, client, dum, dm, pm)
	if err != nil {
		return nil, err
	}
	projectJWTHandler := projectJWT.New(
		options.JWT.Compile, pm.GetEpoch, um.GetEpoch, client,
	)
	loggedInUserJWTHandler := loggedInUserJWT.New(options.JWT.LoggedInUser)
	em := editor.New(
		options,
		editorEvents,
		pm, um,
		chatM, dm,
		projectJWTHandler, loggedInUserJWTHandler,
	)
	lm := login.New(client, um, loggedInUserJWTHandler)
	plm := projectList.New(options, pm, tm, um, loggedInUserJWTHandler)
	pmm := projectMetadata.New(client, editorEvents, pm, dm, dum)
	sm := session.New(options.SessionCookie, client)
	return &manager{
		projectJWTHandler:      projectJWTHandler,
		loggedInUserJWTHandler: loggedInUserJWTHandler,
		compileManager:         cm,
		editorManager:          em,
		loginManager:           lm,
		projectListManager:     plm,
		projectMetadataManager: pmm,
		sessions:               sm,
		systemMessageManager:   smm,
	}, nil
}

type compileManager = compile.Manager
type editorManager = editor.Manager
type loginManager = login.Manager
type projectListManager = projectList.Manager
type projectMetadataManager = projectMetadata.Manager
type sessions = session.Manager
type systemMessageManager = systemMessage.Manager

type manager struct {
	compileManager
	editorManager
	loginManager
	projectListManager
	projectMetadataManager
	sessions
	systemMessageManager
	projectJWTHandler      jwtHandler.JWTHandler
	loggedInUserJWTHandler jwtHandler.JWTHandler
}

func (m *manager) GetProjectJWTHandler() jwtHandler.JWTHandler {
	return m.projectJWTHandler
}

func (m *manager) GetLoggedInUserJWTHandler() jwtHandler.JWTHandler {
	return m.loggedInUserJWTHandler
}
