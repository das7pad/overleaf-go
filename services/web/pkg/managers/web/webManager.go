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

package web

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/jwt/jwtHandler"
	"github.com/das7pad/overleaf-go/pkg/jwt/loggedInUserJWT"
	"github.com/das7pad/overleaf-go/pkg/jwt/projectJWT"
	"github.com/das7pad/overleaf-go/pkg/models/message"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	tagModel "github.com/das7pad/overleaf-go/pkg/models/tag"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/pubSub/channel"
	"github.com/das7pad/overleaf-go/pkg/session"
	"github.com/das7pad/overleaf-go/pkg/templates"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	"github.com/das7pad/overleaf-go/services/filestore/pkg/managers/filestore"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/admin"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/betaProgram"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/compile"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/editor"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/fileTree"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/healthCheck"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/history"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/learn"
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
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/spelling"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/systemMessage"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/tag"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/tokenAccess"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/userCreation"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/userDeletion"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	Cron(ctx context.Context, dryRun bool) bool
	GetPublicSettings() *templates.PublicSettings
	GetProjectJWTHandler() jwtHandler.JWTHandler
	GetLoggedInUserJWTHandler() jwtHandler.JWTHandler

	adminManager
	betaProgramManager
	compileManager
	editorManager
	fileTreeManager
	healthCheckManager
	historyManager
	learnManager
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
	spellingManager
	systemMessageManager
	tagManager
	tokenAccessManager
	userCreationManager
	userDeletionManager
}

func New(options *types.Options, db *sql.DB, client redis.UniversalClient, localURL string) (Manager, error) {
	if err := options.Validate(); err != nil {
		return nil, errors.Tag(err, "invalid options")
	}
	ps, err := options.PublicSettings()
	if err != nil {
		return nil, err
	}
	sm := session.New(options.SessionCookie, client)
	proxy := linkedURLProxy.New(options)
	editorEvents := channel.NewWriter(client, "editor-events")
	mm := message.New(db)
	dum, err := documentUpdater.New(
		options.APIs.DocumentUpdater.Options, db, client,
	)
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
	bm := betaProgram.New(ps, um)
	cm, err := compile.New(options, client, dum, fm, pm, um)
	if err != nil {
		return nil, err
	}
	projectJWTHandler := projectJWT.New(
		options.JWT.Compile, pm.ValidateProjectJWTEpochs,
	)
	loggedInUserJWTHandler := loggedInUserJWT.New(options.JWT.LoggedInUser)
	em := editor.New(
		options, ps,
		client,
		editorEvents,
		pm, um,
		mm, fm,
		projectJWTHandler, loggedInUserJWTHandler,
		cm,
	)
	lm := login.New(options, ps, db, um, loggedInUserJWTHandler, sm)
	plm := projectList.New(ps, editorEvents, pm, tm, um, loggedInUserJWTHandler)
	pmm := projectMetadata.New(client, editorEvents, pm, dum)
	tagM := tag.New(tm)
	tam := tokenAccess.New(ps, pm)
	pim := projectInvite.New(
		options, ps, db, editorEvents, pm, um,
	)
	ftm := fileTree.New(db, pm, dum, fm, editorEvents, pmm)
	pum := projectUpload.New(options, db, pm, um, dum, fm)
	hm, err := history.New(db, client, dum)
	if err != nil {
		return nil, err
	}
	OIOm := openInOverleaf.New(options, ps, proxy, pum)
	lfm, err := linkedFile.New(options, pm, dum, fm, cm, ftm, proxy)
	if err != nil {
		return nil, err
	}
	pdm := projectDownload.New(pm, dum, fm)
	pDelM := projectDeletion.New(pm, dum, fm)
	uDelM := userDeletion.New(db, pm, um, pDelM)
	ucm := userCreation.New(options, ps, db, um, lm)
	am := admin.New(ps, db)
	learnM, err := learn.New(options, ps, proxy)
	if err != nil {
		return nil, err
	}
	hcm, err := healthCheck.New(options, client, um, localURL)
	if err != nil {
		return nil, err
	}
	spm := spelling.New(um)
	return &manager{
		adminManager:           am,
		betaProgramManager:     bm,
		compileManager:         cm,
		editorManager:          em,
		fileTreeManager:        ftm,
		healthCheckManager:     hcm,
		historyManager:         hm,
		learnManager:           learnM,
		linkedFileManager:      lfm,
		loggedInUserJWTHandler: loggedInUserJWTHandler,
		loginManager:           lm,
		notificationsManager:   nm,
		openInOverleafManager:  OIOm,
		projectDeletionManager: pDelM,
		projectDownloadManager: pdm,
		projectInviteManager:   pim,
		projectJWTHandler:      projectJWTHandler,
		projectListManager:     plm,
		projectMetadataManager: pmm,
		projectUploadManager:   pum,
		ps:                     ps,
		sessionManager:         sm,
		spellingManager:        spm,
		systemMessageManager:   smm,
		tagManager:             tagM,
		tokenAccessManager:     tam,
		userCreationManager:    ucm,
		userDeletionManager:    uDelM,
	}, nil
}

type adminManager = admin.Manager
type betaProgramManager = betaProgram.Manager
type compileManager = compile.Manager
type editorManager = editor.Manager
type fileTreeManager = fileTree.Manager
type healthCheckManager = healthCheck.Manager
type historyManager = history.Manager
type learnManager = learn.Manager
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
type spellingManager = spelling.Manager
type systemMessageManager = systemMessage.Manager
type tagManager = tag.Manager
type tokenAccessManager = tokenAccess.Manager
type userCreationManager = userCreation.Manager
type userDeletionManager = userDeletion.Manager

type manager struct {
	adminManager
	betaProgramManager
	compileManager
	editorManager
	fileTreeManager
	healthCheckManager
	historyManager
	learnManager
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
	spellingManager
	systemMessageManager
	tagManager
	tokenAccessManager
	userCreationManager
	userDeletionManager
	loggedInUserJWTHandler jwtHandler.JWTHandler
	projectJWTHandler      jwtHandler.JWTHandler
	ps                     *templates.PublicSettings
}

func (m *manager) GetPublicSettings() *templates.PublicSettings {
	return m.ps
}

func (m *manager) GetProjectJWTHandler() jwtHandler.JWTHandler {
	return m.projectJWTHandler
}

func (m *manager) GetLoggedInUserJWTHandler() jwtHandler.JWTHandler {
	return m.loggedInUserJWTHandler
}

func (m *manager) Cron(ctx context.Context, dryRun bool) bool {
	start := time.Now().UTC()
	ok := true
	if err := m.HardDeleteExpiredProjects(ctx, dryRun, start); err != nil {
		log.Println("hard deletion of projects failed: " + err.Error())
		ok = false
	}
	if err := m.HardDeleteExpiredUsers(ctx, dryRun, start); err != nil {
		log.Println("hard deletion of users failed: " + err.Error())
		ok = false
	}
	return ok
}
