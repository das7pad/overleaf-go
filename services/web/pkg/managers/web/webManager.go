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

package web

import (
	"context"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/mongo"

	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/managers/docstore"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/compile"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/project"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	ClearProjectCache(
		ctx context.Context,
		options types.SignedCompileProjectRequestOptions,
		clsiServerId types.ClsiServerId,
	) error

	CompileProject(
		ctx context.Context,
		request *types.CompileProjectRequest,
		response *types.CompileProjectResponse,
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

func New(options *types.Options, db *mongo.Database, client redis.UniversalClient) (Manager, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	dum, err := documentUpdater.New(options.APIs.DocumentUpdater.Options, client)
	if err != nil {
		return nil, err
	}
	dm, err := docstore.New(options.APIs.Docstore.Options, db)
	if err != nil {
		return nil, err
	}
	pm, err := project.New(db)
	if err != nil {
		return nil, err
	}
	cm, err := compile.New(options, client, dum, dm, pm)
	if err != nil {
		return nil, err
	}
	return &manager{
		cm: cm,
	}, nil
}

type manager struct {
	cm compile.Manager
}

func (m *manager) WordCount(ctx context.Context, request *types.WordCountRequest, words *clsiTypes.Words) error {
	return m.cm.WordCount(ctx, request, words)
}

func (m *manager) SyncFromCode(ctx context.Context, request *types.SyncFromCodeRequest, positions *clsiTypes.PDFPositions) error {
	return m.cm.SyncFromCode(ctx, request, positions)
}

func (m *manager) SyncFromPDF(ctx context.Context, request *types.SyncFromPDFRequest, positions *clsiTypes.CodePositions) error {
	return m.cm.SyncFromPDF(ctx, request, positions)
}

func (m *manager) ClearProjectCache(ctx context.Context, options types.SignedCompileProjectRequestOptions, clsiServerId types.ClsiServerId) error {
	return m.cm.ClearCache(ctx, options, clsiServerId)
}

func (m *manager) CompileProject(ctx context.Context, request *types.CompileProjectRequest, response *types.CompileProjectResponse) error {
	return m.cm.Compile(ctx, request, response)
}
