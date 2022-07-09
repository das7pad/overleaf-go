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

package compile

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	"github.com/das7pad/overleaf-go/services/filestore/pkg/managers/filestore"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"

	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type Manager interface {
	Compile(
		ctx context.Context,
		request *types.CompileProjectRequest,
		response *types.CompileProjectResponse,
	) error

	CompileHeadLess(ctx context.Context, request *types.CompileProjectHeadlessRequest, response *types.CompileProjectResponse) error

	ClearCache(
		ctx context.Context,
		request *types.ClearCompileCacheRequest,
	) error

	StartInBackground(ctx context.Context, options types.SignedCompileProjectRequestOptions, imageName sharedTypes.ImageName) error

	SyncFromCode(
		ctx context.Context,
		request *types.SyncFromCodeRequest,
		positions *clsiTypes.PDFPositions,
	) error

	SyncFromPDF(
		ctx context.Context,
		request *types.SyncFromPDFRequest,
		positions *clsiTypes.CodePositions,
	) error

	WordCount(
		ctx context.Context,
		request *types.WordCountRequest,
		words *clsiTypes.Words,
	) error
}

func New(options *types.Options, client redis.UniversalClient, dum documentUpdater.Manager, fm filestore.Manager, pm project.Manager, um user.Manager) (Manager, error) {
	return &manager{
		baseURL: options.APIs.Clsi.URL.String(),
		options: options,
		client:  client,
		dum:     dum,
		fm:      fm,
		pm:      pm,
		pool:    &http.Client{},
		um:      um,
	}, nil
}

type manager struct {
	baseURL string
	options *types.Options
	client  redis.UniversalClient
	dum     documentUpdater.Manager
	fm      filestore.Manager
	pm      project.Manager
	pool    *http.Client
	um      user.Manager
}

func unexpectedStatus(res *http.Response) error {
	blob, _ := io.ReadAll(res.Body)
	err := errors.New(res.Status + ": " + string(blob))
	return errors.Tag(err, "non-success status code from clsi")
}

func (m *manager) getImageName(raw sharedTypes.ImageName) sharedTypes.ImageName {
	if m.options.TeXLiveImageNameOverride == "" {
		return raw
	}
	idx := strings.LastIndexByte(string(raw), '/')
	return m.options.TeXLiveImageNameOverride + "/" + raw[idx+1:]
}

func (m *manager) ClearCache(ctx context.Context, request *types.ClearCompileCacheRequest) error {
	clearPersistenceError := m.clearServerId(
		ctx, request.SignedCompileProjectRequestOptions,
	)

	u := m.baseURL
	u += "/project/" + request.ProjectId.String()
	u += "/user/" + request.UserId.String()

	r, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return errors.Tag(err, "cannot create clear cache request")
	}
	res, err := m.doStaticRequest(request.ClsiServerId, r)
	if err != nil {
		return errors.Tag(err, "cannot action clear cache request")
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

func (m *manager) StartInBackground(ctx context.Context, options types.SignedCompileProjectRequestOptions, imageName sharedTypes.ImageName) error {
	u := m.baseURL
	u += "/project/" + options.ProjectId.String()
	u += "/user/" + options.UserId.String()
	u += "/status"

	blob, err := json.Marshal(
		clsiTypes.StartInBackgroundRequest{
			ImageName: m.getImageName(imageName),
		},
	)
	body := bytes.NewReader(blob)
	if err != nil {
		return errors.New("cannot serialize start request body")
	}

	r, err := http.NewRequestWithContext(ctx, http.MethodPost, u, body)
	if err != nil {
		return errors.Tag(err, "cannot create start request")
	}
	res, _, err := m.doPersistentRequest(ctx, options, r)
	if err != nil {
		return errors.Tag(err, "cannot action start request")
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

	syncState := request.SyncState

	var resources clsiTypes.Resources
	var rootDocPath clsiTypes.RootResourcePath
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
		if err == nil {
			syncType = clsiTypes.SyncTypeIncremental
		} else if errors.IsInvalidState(err) {
			syncType = clsiTypes.SyncTypeFullIncremental
			fetchContentPerf.Begin()
			resources, rootDocPath, err = m.fromDB(ctx, request)
			fetchContentPerf.End()
			if err != nil {
				return errors.Tag(err, "cannot get docs from db")
			}
		} else {
			return errors.Tag(err, "cannot get docs from redis")
		}

		clsiRequest := &clsiTypes.CompileRequest{
			Options: clsiTypes.CompileOptions{
				Check:        request.CheckMode,
				Compiler:     request.Compiler,
				CompileGroup: request.CompileGroup,
				Draft:        request.Draft,
				ImageName:    request.ImageName,
				SyncState:    syncState,
				SyncType:     syncType,
				Timeout:      request.Timeout,
			},
			Resources:        resources,
			RootResourcePath: rootDocPath,
		}

		err = m.doCompile(ctx, request, clsiRequest, response)
		if err != nil {
			if errors.IsInvalidState(err) && !syncType.IsFull() {
				continue
			}
			return errors.Tag(err, "cannot compile")
		}
		response.Timings.FetchContent = fetchContentPerf
		response.PDFDownloadDomain = m.options.PDFDownloadDomain
		return nil
	}
}

func (m *manager) fromDB(ctx context.Context, request *types.CompileProjectRequest) (clsiTypes.Resources, clsiTypes.RootResourcePath, error) {
	err := m.dum.FlushProject(ctx, request.ProjectId)
	if err != nil {
		return nil, "", errors.Tag(err, "cannot flush docs to db")
	}
	docs, files, err := m.pm.GetProjectWithContent(ctx, request.ProjectId)
	if err != nil {
		return nil, "", errors.Tag(err, "cannot get folder from db")
	}
	rootDocPath := request.RootDocPath
	resources := make(clsiTypes.Resources, 0, 10)

	for _, d := range docs {
		if d.Id == request.RootDocId {
			rootDocPath = clsiTypes.RootResourcePath(d.Path)
		}
		s := d.Snapshot
		resources = append(resources, &clsiTypes.Resource{
			Path:    d.Path,
			Content: &s,
		})
	}
	for _, f := range files {
		url, err2 := m.fm.GetRedirectURLForGETOnProjectFile(
			ctx, request.ProjectId, f.Id,
		)
		if err2 != nil {
			return nil, "", errors.Tag(err, "cannot sign file download")
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

func (m *manager) fromRedis(ctx context.Context, request *types.CompileProjectRequest) (clsiTypes.Resources, clsiTypes.RootResourcePath, error) {
	docs, err := m.dum.GetProjectDocsAndFlushIfOldSnapshot(
		ctx,
		request.ProjectId,
	)
	if err != nil {
		return nil, "", errors.Tag(err, "cannot get docs from redis")
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
			rootDocPath = clsiTypes.RootResourcePath(p)
		}
		resources[i] = &clsiTypes.Resource{
			Path:    p,
			Content: &doc.Snapshot,
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

func (m *manager) doCompile(ctx context.Context, request *types.CompileProjectRequest, requestBody *clsiTypes.CompileRequest, response *types.CompileProjectResponse) error {
	u := m.baseURL
	u += "/project/" + request.ProjectId.String()
	u += "/user/" + request.UserId.String()
	u += "/compile"

	blob, err := json.Marshal(compileRequestBody{Request: requestBody})
	if err != nil {
		return errors.Tag(err, "cannot serialize compile request")
	}

	body := bytes.NewReader(blob)

	r, err := http.NewRequestWithContext(ctx, http.MethodPost, u, body)
	if err != nil {
		return errors.Tag(err, "cannot create compile request")
	}
	res, clsiServerId, err := m.doPersistentRequest(
		ctx, request.SignedCompileProjectRequestOptions, r,
	)
	response.ClsiServerId = clsiServerId
	if err != nil {
		return errors.Tag(err, "cannot action compile request")
	}
	defer func() {
		_ = res.Body.Close()
	}()

	switch res.StatusCode {
	case http.StatusOK:
		responseBody := &compileResponseBody{
			Response: &response.CompileResponse,
		}
		err = json.NewDecoder(res.Body).Decode(responseBody)
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
	default:
		return unexpectedStatus(res)
	}
}
