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

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/jwt/jwtHandler"
	"github.com/das7pad/overleaf-go/pkg/jwt/loggedInUserJWT"
	"github.com/das7pad/overleaf-go/pkg/jwt/projectJWT"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	tagModel "github.com/das7pad/overleaf-go/pkg/models/tag"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/pubSub/channel"
	"github.com/das7pad/overleaf-go/pkg/session"
	"github.com/das7pad/overleaf-go/services/chat/pkg/managers/chat"
	"github.com/das7pad/overleaf-go/services/contacts/pkg/managers/contacts"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/managers/docstore"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	"github.com/das7pad/overleaf-go/services/filestore/pkg/managers/filestore"
	"github.com/das7pad/overleaf-go/services/notifications/pkg/managers/notifications"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/betaProgram"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/compile"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/editor"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/fileTree"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/login"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/projectInvite"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/projectList"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/projectMetadata"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/systemMessage"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/tag"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/tokenAccess"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	GetProjectJWTHandler() jwtHandler.JWTHandler
	GetLoggedInUserJWTHandler() jwtHandler.JWTHandler

	betaProgramManager
	compileManager
	editorManager
	fileTreeManager
	loginManager
	projectInviteManager
	projectListManager
	projectMetadataManager
	sessionManager
	systemMessageManager
	tagManager
	tokenAccessManager
}

func New(options *types.Options, db *mongo.Database, client redis.UniversalClient) (Manager, error) {
	if err := options.Validate(); err != nil {
		return nil, errors.Tag(err, "invalid options")
	}
	editorEvents := channel.NewWriter(client, "editor-events")
	chatM := chat.New(db)
	csm := contacts.New(db)
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
	nm := notifications.New(db)
	pm := project.New(db)
	smm := systemMessage.New(db)
	tm := tagModel.New(db)
	um := user.New(db)
	bm := betaProgram.New(um)
	cm, err := compile.New(options, client, dum, dm, pm)
	if err != nil {
		return nil, err
	}
	fm, err := filestore.New(options.APIs.Filestore.Options)
	if err != nil {
		return nil, err
	}
	projectJWTHandler := projectJWT.New(
		options.JWT.Compile, pm.GetEpoch, um.GetEpoch, client,
	)
	loggedInUserJWTHandler := loggedInUserJWT.New(options.JWT.LoggedInUser)
	em := editor.New(
		options,
		client, db,
		editorEvents,
		pm, tm, um,
		chatM, csm, dm, fm,
		projectJWTHandler, loggedInUserJWTHandler,
	)
	lm := login.New(options, client, um, loggedInUserJWTHandler)
	plm := projectList.New(options, editorEvents, pm, tm, um, loggedInUserJWTHandler)
	pmm := projectMetadata.New(client, editorEvents, pm, dm, dum)
	sm := session.New(options.SessionCookie, client)
	tagM := tag.New(tm)
	tam := tokenAccess.New(client, pm)
	pim := projectInvite.New(options, client, db, editorEvents, pm, um, csm, nm)
	ftm := fileTree.New(db, pm, dm, dum, fm, editorEvents, pmm)
	return &manager{
		projectJWTHandler:      projectJWTHandler,
		loggedInUserJWTHandler: loggedInUserJWTHandler,
		betaProgramManager:     bm,
		compileManager:         cm,
		editorManager:          em,
		fileTreeManager:        ftm,
		loginManager:           lm,
		projectInviteManager:   pim,
		projectListManager:     plm,
		projectMetadataManager: pmm,
		sessionManager:         sm,
		systemMessageManager:   smm,
		tagManager:             tagM,
		tokenAccessManager:     tam,
	}, nil
}

type betaProgramManager = betaProgram.Manager
type compileManager = compile.Manager
type editorManager = editor.Manager
type fileTreeManager = fileTree.Manager
type loginManager = login.Manager
type projectInviteManager = projectInvite.Manager
type projectListManager = projectList.Manager
type projectMetadataManager = projectMetadata.Manager
type sessionManager = session.Manager
type systemMessageManager = systemMessage.Manager
type tagManager = tag.Manager
type tokenAccessManager = tokenAccess.Manager

type manager struct {
	betaProgramManager
	compileManager
	editorManager
	fileTreeManager
	loginManager
	projectInviteManager
	projectListManager
	projectMetadataManager
	sessionManager
	systemMessageManager
	tagManager
	tokenAccessManager
	projectJWTHandler      jwtHandler.JWTHandler
	loggedInUserJWTHandler jwtHandler.JWTHandler
}

func (m *manager) GetProjectJWTHandler() jwtHandler.JWTHandler {
	return m.projectJWTHandler
}

func (m *manager) GetLoggedInUserJWTHandler() jwtHandler.JWTHandler {
	return m.loggedInUserJWTHandler
}
