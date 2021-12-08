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
	"context"
	"log"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/jwt/jwtHandler"
	"github.com/das7pad/overleaf-go/pkg/jwt/loggedInUserJWT"
	"github.com/das7pad/overleaf-go/pkg/jwt/projectJWT"
	"github.com/das7pad/overleaf-go/pkg/models/contact"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	tagModel "github.com/das7pad/overleaf-go/pkg/models/tag"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/pubSub/channel"
	"github.com/das7pad/overleaf-go/pkg/session"
	"github.com/das7pad/overleaf-go/services/chat/pkg/managers/chat"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/managers/docstore"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	"github.com/das7pad/overleaf-go/services/filestore/pkg/managers/filestore"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/betaProgram"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/compile"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/editor"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/fileTree"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/history"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/inactiveProject"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/linkedFile"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/linkedURLProxy"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/login"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/notifications"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/openInOverleaf"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/projectDeletion"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/projectDownload"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/projectInvite"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/projectList"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/projectMetadata"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/projectUpload"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/systemMessage"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/tag"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/tokenAccess"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/userDeletion"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	Cron(ctx context.Context, dryRun bool) bool
	GetProjectJWTHandler() jwtHandler.JWTHandler
	GetLoggedInUserJWTHandler() jwtHandler.JWTHandler

	betaProgramManager
	compileManager
	editorManager
	fileTreeManager
	historyManager
	inactiveProjectManager
	linkedFileManager
	loginManager
	notificationsManager
	openInOverleafManager
	projectDeletionManager
	projectDownloadManager
	projectInviteManager
	projectListManager
	projectMetadataManager
	projectUploadManager
	sessionManager
	systemMessageManager
	tagManager
	tokenAccessManager
	userDeletionManager
}

func New(options *types.Options, db *mongo.Database, client redis.UniversalClient) (Manager, error) {
	if err := options.Validate(); err != nil {
		return nil, errors.Tag(err, "invalid options")
	}
	proxy := linkedURLProxy.New(options)
	editorEvents := channel.NewWriter(client, "editor-events")
	chatM := chat.New(db)
	csm := contact.New(db)
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
	fm, err := filestore.New(options.APIs.Filestore.Options)
	if err != nil {
		return nil, err
	}
	nm := notifications.New(db)
	pm := project.New(db)
	smm := systemMessage.New(db)
	tm := tagModel.New(db)
	um := user.New(db)
	bm := betaProgram.New(um)
	cm, err := compile.New(options, client, dum, dm, fm, pm, um)
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
	plm := projectList.New(editorEvents, pm, tm, um, loggedInUserJWTHandler)
	pmm := projectMetadata.New(client, editorEvents, pm, dm, dum)
	sm := session.New(options.SessionCookie, client)
	tagM := tag.New(tm)
	tam := tokenAccess.New(client, pm)
	pim := projectInvite.New(options, client, db, editorEvents, pm, um, csm)
	ftm := fileTree.New(db, pm, dm, dum, fm, editorEvents, pmm)
	pum := projectUpload.New(options, db, pm, um, dm, dum, fm)
	hm := history.New(options, um)
	OIOm := openInOverleaf.New(options, proxy, pum)
	lfm, err := linkedFile.New(options, pm, dum, fm, cm, ftm, proxy)
	if err != nil {
		return nil, err
	}
	pdm := projectDownload.New(pm, dm, dum, fm)
	pDelM := projectDeletion.New(db, pm, tm, chatM, dm, dum, fm)
	uDelM := userDeletion.New(db, pm, um, tm, csm, pDelM)
	ipm := inactiveProject.New(options, pm, dm)
	return &manager{
		projectJWTHandler:      projectJWTHandler,
		loggedInUserJWTHandler: loggedInUserJWTHandler,
		betaProgramManager:     bm,
		compileManager:         cm,
		editorManager:          em,
		fileTreeManager:        ftm,
		historyManager:         hm,
		inactiveProjectManager: ipm,
		linkedFileManager:      lfm,
		loginManager:           lm,
		notificationsManager:   nm,
		openInOverleafManager:  OIOm,
		projectDeletionManager: pDelM,
		projectDownloadManager: pdm,
		projectInviteManager:   pim,
		projectListManager:     plm,
		projectMetadataManager: pmm,
		projectUploadManager:   pum,
		sessionManager:         sm,
		systemMessageManager:   smm,
		tagManager:             tagM,
		tokenAccessManager:     tam,
		userDeletionManager:    uDelM,
	}, nil
}

type betaProgramManager = betaProgram.Manager
type compileManager = compile.Manager
type editorManager = editor.Manager
type fileTreeManager = fileTree.Manager
type historyManager = history.Manager
type inactiveProjectManager = inactiveProject.Manager
type linkedFileManager = linkedFile.Manager
type loginManager = login.Manager
type notificationsManager = notifications.Manager
type openInOverleafManager = openInOverleaf.Manager
type projectDeletionManager = projectDeletion.Manager
type projectDownloadManager = projectDownload.Manager
type projectInviteManager = projectInvite.Manager
type projectListManager = projectList.Manager
type projectMetadataManager = projectMetadata.Manager
type projectUploadManager = projectUpload.Manager
type sessionManager = session.Manager
type systemMessageManager = systemMessage.Manager
type tagManager = tag.Manager
type tokenAccessManager = tokenAccess.Manager
type userDeletionManager = userDeletion.Manager

type manager struct {
	betaProgramManager
	compileManager
	editorManager
	fileTreeManager
	historyManager
	inactiveProjectManager
	linkedFileManager
	loginManager
	notificationsManager
	openInOverleafManager
	projectDeletionManager
	projectDownloadManager
	projectInviteManager
	projectListManager
	projectMetadataManager
	projectUploadManager
	sessionManager
	systemMessageManager
	tagManager
	tokenAccessManager
	userDeletionManager
	projectJWTHandler      jwtHandler.JWTHandler
	loggedInUserJWTHandler jwtHandler.JWTHandler
}

func (m *manager) GetProjectJWTHandler() jwtHandler.JWTHandler {
	return m.projectJWTHandler
}

func (m *manager) GetLoggedInUserJWTHandler() jwtHandler.JWTHandler {
	return m.loggedInUserJWTHandler
}

func (m *manager) Cron(ctx context.Context, dryRun bool) bool {
	ok := true
	if err := m.HardDeleteExpiredProjects(ctx, dryRun); err != nil {
		log.Println("hard deletion of projects failed: " + err.Error())
		ok = false
	}
	if err := m.HardDeleteExpiredUsers(ctx, dryRun); err != nil {
		log.Println("hard deletion of users failed: " + err.Error())
		ok = false
	}
	if err := m.ArchiveOldProjects(ctx, dryRun); err != nil {
		log.Println("hard deletion of users failed: " + err.Error())
		ok = false
	}
	return ok
}
