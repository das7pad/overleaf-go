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

package compile

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/pendingOperation"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	"github.com/das7pad/overleaf-go/services/filestore/pkg/managers/filestore"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"

	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type Manager interface {
	Compile(ctx context.Context, request *types.CompileProjectRequest, response *types.CompileProjectResponse) error
	CompileHeadLess(ctx context.Context, request *types.CompileProjectHeadlessRequest, response *types.CompileProjectResponse) error
	ClearCache(ctx context.Context, request *types.ClearCompileCacheRequest) error
	StartInBackground(ctx context.Context, options sharedTypes.ProjectOptions, imageName sharedTypes.ImageName) error
	SyncFromCode(ctx context.Context, request *types.SyncFromCodeRequest, response *types.SyncFromCodeResponse) error
	SyncFromPDF(ctx context.Context, request *types.SyncFromPDFRequest, response *types.SyncFromPDFResponse) error
	WordCount(ctx context.Context, request *types.WordCountRequest, response *types.WordCountResponse) error
}

func New(options *types.Options, client redis.UniversalClient, dum documentUpdater.Manager, fm filestore.Manager, pm project.Manager, um user.Manager, bundle ClsiManager) (Manager, error) {
	return &manager{
		bundle:                   bundle,
		baseURL:                  options.APIs.Clsi.URL,
		persistenceCookieName:    options.APIs.Clsi.Persistence.CookieName,
		persistenceTTL:           options.APIs.Clsi.Persistence.TTL,
		pdfDownloadDomain:        options.PDFDownloadDomain,
		teXLiveImageNameOverride: options.TeXLiveImageNameOverride,
		client:                   client,
		dum:                      dum,
		fm:                       fm,
		pm:                       pm,
		pool:                     &http.Client{},
		um:                       um,
	}, nil
}

type manager struct {
	bundle                   ClsiManager
	baseURL                  sharedTypes.URL
	persistenceCookieName    string
	persistenceTTL           time.Duration
	pdfDownloadDomain        types.PDFDownloadDomain
	teXLiveImageNameOverride sharedTypes.ImageName
	client                   redis.UniversalClient
	dum                      documentUpdater.Manager
	fm                       filestore.Manager
	pm                       project.Manager
	pool                     *http.Client
	um                       user.Manager
}

func unexpectedStatus(res *http.Response) error {
	blob, _ := io.ReadAll(res.Body)
	err := errors.New(res.Status + ": " + string(blob))
	return errors.Tag(err, "non-success status code from clsi")
}

func (m *manager) getImageName(raw sharedTypes.ImageName) sharedTypes.ImageName {
	if m.teXLiveImageNameOverride == "" {
		return raw
	}
	idx := strings.LastIndexByte(string(raw), '/')
	return m.teXLiveImageNameOverride + "/" + raw[idx+1:]
}

func (m *manager) getURL(projectId, userId sharedTypes.UUID, endpoint string) string {
	return m.baseURL.WithPath(
		"" +
			"/project/" + projectId.String() +
			"/user/" + userId.String() +
			endpoint,
	).String()
}

func (m *manager) ClearCache(ctx context.Context, request *types.ClearCompileCacheRequest) error {
	if m.bundle != nil {
		return m.bundle.ClearCache(request.ProjectId, request.UserId)
	}
	clearPersistenceError := m.clearServerId(ctx, request.ProjectOptions)

	u := m.getURL(request.ProjectId, request.UserId, "")

	r, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return errors.Tag(err, "create clear cache request")
	}
	res, err := m.doStaticRequest(request.ClsiServerId, r)
	if err != nil {
		return errors.Tag(err, "action clear cache request")
	}
	defer func() {
		_ = res.Body.Close()
	}()

	switch res.StatusCode {
	case http.StatusNoContent:
		if clearPersistenceError != nil {
			return clearPersistenceError
		}
		return nil
	default:
		return unexpectedStatus(res)
	}
}

func (m *manager) StartInBackground(ctx context.Context, options sharedTypes.ProjectOptions, imageName sharedTypes.ImageName) error {
	request := clsiTypes.StartInBackgroundRequest{
		ImageName: m.getImageName(imageName),
	}
	if m.bundle != nil {
		return m.bundle.StartInBackground(
			ctx, options.ProjectId, options.UserId, &request,
		)
	}

	u := m.getURL(options.ProjectId, options.UserId, "/status")

	blob, err := json.Marshal(request)
	if err != nil {
		return errors.Tag(err, "serialize start request body")
	}

	body := bytes.NewReader(blob)
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, u, body)
	if err != nil {
		return errors.Tag(err, "create start request")
	}
	clsiServerId, err := m.getServerId(ctx, options)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		log.Printf("get clsi persistence: %s", err)
	}
	res, _, err := m.doPersistentRequest(ctx, options, clsiServerId, r)
	if err != nil {
		return errors.Tag(err, "action start request")
	}
	defer func() {
		_ = res.Body.Close()
	}()

	switch res.StatusCode {
	case http.StatusOK:
		return nil
	default:
		return unexpectedStatus(res)
	}
}

func (m *manager) Compile(ctx context.Context, request *types.CompileProjectRequest, response *types.CompileProjectResponse) error {
	if err := request.Validate(); err != nil {
		return err
	}
	request.ImageName = m.getImageName(request.ImageName)

	var clsiServerId types.ClsiServerId
	pendingFetchClsiServerId := pendingOperation.TrackOperationWithCancel(
		ctx,
		func(ctx context.Context) error {
			var err error
			clsiServerId, err = m.getServerId(ctx, request.ProjectOptions)
			return err
		},
	)
	defer pendingFetchClsiServerId.Cancel()

	var resources clsiTypes.Resources
	var rootDocPath sharedTypes.PathName
	var err error
	fetchContentPerf := response.Timings.FetchContent
	if request.IncrementalCompilesEnabled {
		fetchContentPerf.Begin()
		resources, rootDocPath, err = m.fromRedis(ctx, request)
		fetchContentPerf.End()
	} else {
		err = &errors.InvalidStateError{}
	}
	var syncType clsiTypes.SyncType

	for {
		switch {
		case err == nil:
			syncType = clsiTypes.SyncTypeIncremental
		case errors.IsInvalidStateError(err):
			syncType = clsiTypes.SyncTypeFullIncremental
			fetchContentPerf.Begin()
			resources, rootDocPath, err = m.fromDB(ctx, request)
			fetchContentPerf.End()
			if err != nil {
				return errors.Tag(err, "get docs from db")
			}
		default:
			return errors.Tag(err, "get docs from redis")
		}

		clsiRequest := clsiTypes.CompileRequest{
			Options: clsiTypes.CompileOptions{
				Check:            request.CheckMode,
				Compiler:         request.Compiler,
				CompileGroup:     request.CompileGroup,
				Draft:            request.Draft,
				ImageName:        request.ImageName,
				RootResourcePath: rootDocPath,
				SyncState:        request.SyncState,
				SyncType:         syncType,
				Timeout:          request.Timeout,
			},
			Resources: resources,
		}

		if err = pendingFetchClsiServerId.Wait(ctx); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log.Printf("get clsi persistence: %s", err)
		}
		if m.bundle != nil {
			err = m.bundle.Compile(
				ctx, request.ProjectId, request.UserId, &clsiRequest,
				&response.CompileResponse,
			)
		} else {
			err = m.doCompile(
				ctx, request, clsiServerId, &clsiRequest, response,
			)
			clsiServerId = response.ClsiServerId
		}
		response.Timings.FetchContent = fetchContentPerf
		response.PDFDownloadDomain = m.pdfDownloadDomain
		if err != nil {
			if errors.IsInvalidStateError(err) && !syncType.IsFull() {
				continue // retry as full sync
			}
			if errors.IsAlreadyCompilingError(err) {
				response.Status = "compile-in-progress"
				return nil
			}
			if errors.IsCompilerUnavailableError(err) {
				response.Status = "clsi-maintenance"
				return nil
			}
			return errors.Tag(err, "compile")
		}
		return nil
	}
}

func (m *manager) fromDB(ctx context.Context, request *types.CompileProjectRequest) (clsiTypes.Resources, sharedTypes.PathName, error) {
	err := m.dum.FlushProject(ctx, request.ProjectId)
	if err != nil {
		return nil, "", errors.Tag(err, "flush docs to db")
	}
	docs, files, err := m.pm.GetProjectWithContent(ctx, request.ProjectId)
	if err != nil {
		return nil, "", errors.Tag(err, "get files from db")
	}
	rootDocPath := request.RootDocPath
	resources := make(clsiTypes.Resources, 0, 10)

	for _, d := range docs {
		if d.Id == request.RootDocId {
			rootDocPath = d.Path
		}
		resources = append(resources, &clsiTypes.Resource{
			Path:    d.Path,
			Content: d.Snapshot,
			Version: d.Version,
		})
	}
	for _, f := range files {
		url, err2 := m.fm.GetRedirectURLForGETOnProjectFile(
			ctx, request.ProjectId, f.Id,
		)
		if err2 != nil {
			return nil, "", errors.Tag(err, "sign file download")
		}
		resources = append(resources, &clsiTypes.Resource{
			Path: f.Path,
			URL:  &sharedTypes.URL{URL: *url},
		})
	}
	if rootDocPath == "" {
		return nil, "", &errors.ValidationError{Msg: "rootDoc not found"}
	}
	return resources, rootDocPath, nil
}

func (m *manager) fromRedis(ctx context.Context, request *types.CompileProjectRequest) (clsiTypes.Resources, sharedTypes.PathName, error) {
	docs, err := m.dum.GetProjectDocsWithRootDocAndFlushIfOld(
		ctx, request.ProjectId, request.RootDocId,
	)
	if err != nil {
		return nil, "", errors.Tag(err, "get docs from redis")
	}
	if len(docs) == 0 {
		return nil, "", &errors.InvalidStateError{Msg: "no docs found"}
	}

	rootDocPath := request.RootDocPath
	resources := make(clsiTypes.Resources, len(docs))
	for i, doc := range docs {
		p := doc.PathName
		if p != "" {
			// Paths used to be stored absolute in redis :/
			if p[0] == '/' {
				p = p[1:]
			}
		}
		if doc.Id == request.RootDocId {
			rootDocPath = p
		}
		resources[i] = &clsiTypes.Resource{
			Path:    p,
			Content: doc.Snapshot,
			Version: doc.Version,
		}
	}
	if rootDocPath == "" {
		return nil, "", &errors.ValidationError{Msg: "rootDoc not found"}
	}
	return resources, rootDocPath, nil
}

type compileRequestBody struct {
	Request *clsiTypes.CompileRequest `json:"compile"`
}
type compileResponseBody struct {
	Response *clsiTypes.CompileResponse `json:"compile"`
}

func (m *manager) doCompile(ctx context.Context, request *types.CompileProjectRequest, clsiServerId types.ClsiServerId, requestBody *clsiTypes.CompileRequest, response *types.CompileProjectResponse) error {
	u := m.getURL(request.ProjectId, request.UserId, "/compile")

	blob, err := json.Marshal(compileRequestBody{Request: requestBody})
	if err != nil {
		return errors.Tag(err, "serialize compile request")
	}

	body := bytes.NewReader(blob)

	r, err := http.NewRequestWithContext(ctx, http.MethodPost, u, body)
	if err != nil {
		return errors.Tag(err, "create compile request")
	}
	res, clsiServerId, err := m.doPersistentRequest(
		ctx, request.ProjectOptions, clsiServerId, r,
	)
	response.ClsiServerId = clsiServerId
	if err != nil {
		return errors.Tag(err, "action compile request")
	}
	defer func() {
		_ = res.Body.Close()
	}()

	switch res.StatusCode {
	case http.StatusOK:
		responseBody := compileResponseBody{
			Response: &response.CompileResponse,
		}
		err = json.NewDecoder(res.Body).Decode(&responseBody)
		if err != nil {
			return err
		}
		return nil
	case http.StatusBadRequest:
		return &errors.ValidationError{}
	case http.StatusNotFound:
		return &errors.NotFoundError{}
	case http.StatusConflict:
		return &errors.InvalidStateError{}
	case http.StatusLocked:
		return &errors.AlreadyCompilingError{}
	case http.StatusServiceUnavailable:
		return &errors.CompilerUnavailableError{}
	default:
		return unexpectedStatus(res)
	}
}
