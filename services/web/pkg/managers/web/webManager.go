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

package web

import (
	"context"
	"log"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/das7pad/overleaf-go/pkg/assets"
	"github.com/das7pad/overleaf-go/pkg/errors"
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
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/siteLanguage"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/spelling"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/systemMessage"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/tag"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/tokenAccess"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/userCreation"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/userDeletion"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	Cron(ctx context.Context, dryRun bool, initialJitter time.Duration)
	CronOnce(ctx context.Context, dryRun bool) bool
	GetPublicSettings() *templates.PublicSettings
	GetProjectJWTHandler() *projectJWT.JWTHandler
	GetLoggedInUserJWTHandler() *loggedInUserJWT.JWTHandler
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
	siteLanguageManager
	spellingManager
	systemMessageManager
	tagManager
	tokenAccessManager
	userCreationManager
	userDeletionManager
}

func New(options *types.Options, db *pgxpool.Pool, client redis.UniversalClient, localURL string, dum documentUpdater.Manager, clsiBundle compile.ClsiManager) (Manager, error) {
	if err := options.Validate(); err != nil {
		return nil, errors.Tag(err, "invalid options")
	}
	proxy, err := linkedURLProxy.New(options)
	if err != nil {
		return nil, err
	}

	am, err := assets.Load(options.AssetsOptions(), proxy)
	if err != nil {
		return nil, errors.Tag(err, "load assets")
	}
	if err = templates.Load(options.AppName, options.I18n, am); err != nil {
		return nil, errors.Tag(err, "load templates")
	}

	ps, err := options.PublicSettings()
	if err != nil {
		return nil, err
	}
	options.SessionCookie.Secure = options.SiteURL.Scheme == "https"
	sm := session.New(options.SessionCookie, client)
	editorEvents := channel.NewWriter(client, "editor-events")
	mm := message.New(db)
	fm, err := filestore.New(options.APIs.Filestore)
	if err != nil {
		return nil, err
	}
	nm := notifications.New(db)
	pm := project.New(db)
	smm := systemMessage.New(db)
	tm := tagModel.New(db)
	um := user.New(db)
	bm := betaProgram.New(ps, um)
	cm, err := compile.New(options, client, dum, fm, pm, um, clsiBundle)
	if err != nil {
		return nil, err
	}
	projectJWTHandler := projectJWT.New(
		options.JWT.Project, pm.ValidateProjectJWTEpochs,
	)
	loggedInUserJWTHandler := loggedInUserJWT.New(options.JWT.LoggedInUser)
	em := editor.New(
		options, ps,
		client,
		editorEvents,
		pm, um,
		mm, fm,
		projectJWTHandler, loggedInUserJWTHandler,
		dum,
		cm, smm,
	)
	lm := login.New(options, ps, db, um, loggedInUserJWTHandler, sm)
	plm := projectList.New(
		ps, editorEvents, pm, tm, um, loggedInUserJWTHandler, smm,
	)
	pmm := projectMetadata.New(client, editorEvents, pm, dum)
	tagM := tag.New(tm)
	tam := tokenAccess.New(options, ps, pm)
	pim := projectInvite.New(
		options, ps, db, editorEvents, pm, um,
	)
	ftm := fileTree.New(pm, dum, fm, editorEvents, pmm)
	pum := projectUpload.New(options, pm, um, dum, fm)
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
	uDelM := userDeletion.New(um, pDelM)
	ucm := userCreation.New(options, ps, db, um, lm)
	learnM, err := learn.New(options, ps, proxy)
	if err != nil {
		return nil, err
	}
	hcm, err := healthCheck.New(options, client, um, localURL)
	if err != nil {
		return nil, err
	}
	spm := spelling.New(um)
	slm := siteLanguage.New(options)
	return &manager{
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
		siteLanguageManager:    slm,
		spellingManager:        spm,
		systemMessageManager:   smm,
		tagManager:             tagM,
		tokenAccessManager:     tam,
		userCreationManager:    ucm,
		userDeletionManager:    uDelM,
	}, nil
}

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

type siteLanguageManager = siteLanguage.Manager

type userCreationManager = userCreation.Manager

type userDeletionManager = userDeletion.Manager

type manager struct {
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
	siteLanguageManager
	spellingManager
	systemMessageManager
	tagManager
	tokenAccessManager
	userCreationManager
	userDeletionManager
	loggedInUserJWTHandler *loggedInUserJWT.JWTHandler
	projectJWTHandler      *projectJWT.JWTHandler
	ps                     *templates.PublicSettings
}

func (m *manager) GetPublicSettings() *templates.PublicSettings {
	return m.ps
}

func (m *manager) GetProjectJWTHandler() *projectJWT.JWTHandler {
	return m.projectJWTHandler
}

func (m *manager) GetLoggedInUserJWTHandler() *loggedInUserJWT.JWTHandler {
	return m.loggedInUserJWTHandler
}

func (m *manager) Cron(ctx context.Context, dryRun bool, jitter time.Duration) {
	for {
		t := time.NewTimer(
			14*time.Minute + time.Duration(rand.Int63n(int64(jitter))),
		)
		jitter = time.Minute
		select {
		case <-ctx.Done():
			if !t.Stop() {
				<-t.C
			}
			return
		case <-t.C:
		}

		m.CronOnce(ctx, dryRun)
	}
}

func (m *manager) CronOnce(ctx context.Context, dryRun bool) bool {
	start := time.Now()
	ok := true
	if err := m.HardDeleteExpiredProjects(ctx, dryRun, start); err != nil {
		log.Println("hard deletion of projects failed: " + err.Error())
		ok = false
	}
	if err := m.HardDeleteExpiredUsers(ctx, dryRun, start); err != nil {
		log.Println("hard deletion of users failed: " + err.Error())
		ok = false
	}
	if err := m.CleanupStaleFileUploads(ctx, dryRun, start); err != nil {
		log.Println("purging of file uploads failed: " + err.Error())
		ok = false
	}
	return ok
}
