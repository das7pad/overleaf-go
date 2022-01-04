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

package compile

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/managers/docstore"
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
		options types.SignedCompileProjectRequestOptions,
		clsiServerId types.ClsiServerId,
	) error

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

func New(options *types.Options, client redis.UniversalClient, dum documentUpdater.Manager, dm docstore.Manager, fm filestore.Manager, pm project.Manager, um user.Manager) (Manager, error) {
	return &manager{
		baseURL: options.APIs.Clsi.URL.String(),
		options: options,
		client:  client,
		dum:     dum,
		dm:      dm,
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
	dm      docstore.Manager
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

func (m *manager) ClearCache(ctx context.Context, request types.SignedCompileProjectRequestOptions, clsiServerId types.ClsiServerId) error {
	clearPersistenceError := m.clearServerId(ctx, request)

	u := m.baseURL
	u += "/project/" + request.ProjectId.Hex()
	u += "/user/" + request.UserId.Hex()

	r, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return errors.Tag(err, "cannot create clear cache request")
	}
	res, err := m.doStaticRequest(clsiServerId, r)
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
			resources, rootDocPath, err = m.fromMongo(ctx, request)
			fetchContentPerf.End()
			if err != nil {
				return errors.Tag(err, "cannot get docs from mongo")
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

func (m *manager) fromMongo(ctx context.Context, request *types.CompileProjectRequest) (clsiTypes.Resources, clsiTypes.RootResourcePath, error) {
	err := m.dum.FlushProject(ctx, request.ProjectId)
	if err != nil {
		return nil, "", errors.Tag(err, "cannot flush docs to mongo")
	}
	docContents, err := m.dm.GetAllDocContents(ctx, request.ProjectId)
	if err != nil {
		return nil, "", errors.Tag(err, "cannot get docs from mongo")
	}

	folder, _, err := m.pm.GetProjectRootFolder(ctx, request.ProjectId)
	if err != nil {
		return nil, "", errors.Tag(err, "cannot get folder from mongo")
	}
	files := make(clsiTypes.Resources, 0)
	docs := make(map[primitive.ObjectID]sharedTypes.PathName, 0)

	err = folder.Walk(func(e project.TreeElement, p sharedTypes.PathName) error {
		switch entry := e.(type) {
		case *project.Doc:
			docs[entry.Id] = p
		case *project.FileRef:
			t := clsiTypes.ModifiedAt(entry.Created.Unix())
			url, err2 := m.fm.GetRedirectURLForGETOnProjectFile(
				ctx, request.ProjectId, entry.Id,
			)
			if err2 != nil {
				return errors.Tag(err, "cannot sign file download")
			}
			files = append(files, &clsiTypes.Resource{
				Path:       p,
				ModifiedAt: &t,
				URL:        &sharedTypes.URL{URL: *url},
			})
		}
		return nil
	})
	if err != nil {
		return nil, "", errors.Tag(err, "cannot walk folder")
	}

	rootDocPath := request.RootDocPath
	resources := make(clsiTypes.Resources, len(docContents)+len(files))
	copy(resources[len(docContents):], files)

	for i, doc := range docContents {
		p, exists := docs[doc.Id]
		if !exists {
			return nil, "", errors.Tag(
				&errors.NotFoundError{}, "cannot find doc "+doc.Id.Hex(),
			)
		}

		if doc.Id == request.RootDocId {
			rootDocPath = clsiTypes.RootResourcePath(p)
		}

		s := doc.Lines.ToSnapshot()
		resources[i] = &clsiTypes.Resource{
			Path:    p,
			Content: &s,
		}
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
	u += "/project/" + request.ProjectId.Hex()
	u += "/user/" + request.UserId.Hex()
	u += "/compile"

	blob, err := json.Marshal(&compileRequestBody{Request: requestBody})
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
	// TODO: convert other codes into proper errors
	case http.StatusConflict:
		return &errors.InvalidStateError{}
	default:
		return unexpectedStatus(res)
	}
}
