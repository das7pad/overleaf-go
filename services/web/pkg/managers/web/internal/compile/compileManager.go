// Golang port of the Overleaf web service
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
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/managers/docstore"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/project"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"

	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type Manager interface {
	Compile(
		ctx context.Context,
		request *types.CompileProjectRequest,
		response *types.CompileProjectResponse,
	) error
}

func New(options *types.Options, client redis.UniversalClient, dum documentUpdater.Manager, dm docstore.Manager, pm project.Manager) (Manager, error) {
	return &manager{
		baseURL: options.APIs.Clsi.URL.String(),
		options: options,
		client:  client,
		dum:     dum,
		dm:      dm,
		pm:      pm,
		pool:    &http.Client{},
	}, nil
}

type manager struct {
	baseURL string
	options *types.Options
	client  redis.UniversalClient
	dum     documentUpdater.Manager
	dm      docstore.Manager
	pm      project.Manager
	pool    *http.Client
}

func (m *manager) Compile(ctx context.Context, request *types.CompileProjectRequest, response *types.CompileProjectResponse) error {
	if m.options.TeXLiveImageNameOverride != "" {
		idx := strings.LastIndexByte(string(request.ImageName), '/')
		request.ImageName = m.options.TeXLiveImageNameOverride + "/" +
			request.ImageName[idx+1:]
	}

	syncState := clsiTypes.SyncState("TODO")

	var resources clsiTypes.Resources
	var rootDocPath clsiTypes.RootResourcePath
	var err error
	if request.IncrementalCompilesEnabled {
		resources, rootDocPath, err = m.fromRedis(ctx, request)
	} else {
		err = &errors.InvalidStateError{}
	}
	var syncType clsiTypes.SyncType

	for {
		if err == nil {
			syncType = clsiTypes.SyncTypeIncremental
		} else if errors.IsInvalidState(err) {
			syncType = clsiTypes.SyncTypeFullIncremental
			resources, rootDocPath, err = m.fromMongo(ctx, request)
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

	folder, err := m.pm.GetProjectRootFolder(ctx, request.ProjectId)
	if err != nil {
		return nil, "", errors.Tag(err, "cannot get folder from mongo")
	}
	files := make(clsiTypes.Resources, 0)
	docs := make(map[primitive.ObjectID]sharedTypes.PathName, 0)

	err = folder.Walk(func(e types.TreeElement, p sharedTypes.PathName) error {
		switch entry := e.(type) {
		case types.Doc:
			docs[entry.Id] = p
		case types.FileRef:
			t := clsiTypes.ModifiedAt(entry.Created.Unix())
			url := m.options.APIs.Filestore.URL.WithPath(
				"/project/" + request.ProjectId.Hex() +
					"/file/" + entry.Id.Hex(),
			)
			files = append(files, &clsiTypes.Resource{
				Path:       p,
				ModifiedAt: &t,
				URL:        &url,
			})
		}
		return nil
	})
	if err != nil {
		return nil, "", errors.Tag(err, "cannot walk folder")
	}

	var rootDocPath clsiTypes.RootResourcePath
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
	syncState := clsiTypes.SyncState("TODO")
	docs, err := m.dum.GetProjectDocsAndFlushIfOldSnapshot(
		ctx,
		request.ProjectId,
		string(syncState),
	)
	if err != nil {
		return nil, "", errors.Tag(err, "cannot get docs from redis")
	}

	resources := make(clsiTypes.Resources, len(docs))
	var rootDocPath clsiTypes.RootResourcePath
	for i, doc := range docs {
		p := doc.PathName
		if p != "" {
			// Paths are stored absolute in redis :/
			p = p[1:]
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

func getPersistenceKey(compileGroup clsiTypes.CompileGroup, projectId, userId primitive.ObjectID) string {
	return fmt.Sprintf("clsiserver:%s:%s:%s", compileGroup, projectId.Hex(), userId.Hex())
}

func (m *manager) populateServerIdFromResponse(ctx context.Context, res *http.Response, compileGroup clsiTypes.CompileGroup, projectId, userId primitive.ObjectID) (types.ClsiServerId, error) {
	if m.options.APIs.Clsi.Persistence.CookieName == "" {
		return "", nil
	}
	var clsiServerId types.ClsiServerId
	for _, cookie := range res.Cookies() {
		if cookie.Name == m.options.APIs.Clsi.Persistence.CookieName {
			clsiServerId = types.ClsiServerId(cookie.Value)
			break
		}
	}
	k := getPersistenceKey(compileGroup, projectId, userId)
	persistenceTTL := m.options.APIs.Clsi.Persistence.TTL
	var err error
	if clsiServerId == "" {
		err = m.client.Expire(ctx, k, persistenceTTL).Err()
	} else {
		err = m.client.Set(ctx, k, string(clsiServerId), persistenceTTL).Err()
	}
	return clsiServerId, err
}

func (m *manager) assignNewServerId(ctx context.Context, compileGroup clsiTypes.CompileGroup, projectId, userId primitive.ObjectID) (types.ClsiServerId, error) {
	u := m.baseURL
	u += "/project/" + projectId.Hex()
	u += "/user/" + userId.Hex()
	u += "/status"
	r, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		u,
		nil,
	)
	if err != nil {
		return "", errors.Tag(err, "cannot create cookie status req")
	}
	res, err := m.pool.Do(r)
	if err != nil {
		return "", errors.Tag(err, "cannot perform cookie status req")
	}

	return m.populateServerIdFromResponse(ctx, res, compileGroup, projectId, userId)
}

func (m *manager) getServerId(ctx context.Context, compileGroup clsiTypes.CompileGroup, projectId, userId primitive.ObjectID) (types.ClsiServerId, error) {
	if m.options.APIs.Clsi.Persistence.CookieName == "" {
		return "", nil
	}
	k := getPersistenceKey(compileGroup, projectId, userId)
	s, err := m.client.Get(ctx, k).Result()
	if err != nil && err != redis.Nil {
		return "", err
	}
	if s != "" {
		return types.ClsiServerId(s), nil
	}
	return m.assignNewServerId(ctx, compileGroup, projectId, userId)
}

func (m *manager) doCompile(ctx context.Context, request *types.CompileProjectRequest, requestBody *clsiTypes.CompileRequest, response *types.CompileProjectResponse) error {
	clsiServerId, err := m.getServerId(ctx, request.CompileGroup, request.ProjectId, request.UserId)
	if err != nil {
		return err
	}

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
		return err
	}
	if clsiServerId != "" {
		response.ClsiServerId = clsiServerId
		r.AddCookie(&http.Cookie{
			Name:  m.options.APIs.Clsi.Persistence.CookieName,
			Value: string(clsiServerId),
		})
	}
	res, err := m.pool.Do(r)
	if err != nil {
		return err
	}
	defer func() {
		_ = res.Body.Close()
	}()

	newClsiServerId, err := m.populateServerIdFromResponse(
		ctx,
		res,
		request.CompileGroup,
		request.ProjectId,
		request.UserId,
	)
	if err != nil {
		// This is semi-ok to fail. We got a response, why discard it now?
		log.Printf("cannot update clsi persistence: %s", err.Error())
	}
	if newClsiServerId != "" {
		response.ClsiServerId = newClsiServerId
	}

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
		blob, err = io.ReadAll(res.Body)
		return errors.New(
			"non-success status code from clsi: " + res.Status + ": " + string(blob),
		)
	}
}
