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

	"github.com/das7pad/overleaf-go/services/docstore/pkg/managers/docstore"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/compile"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/project"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	CompileProject(
		ctx context.Context,
		request *types.CompileProjectRequest,
		response *types.CompileProjectResponse,
	) error
}

func New(options *types.Options, db *mongo.Database, client redis.UniversalClient) (Manager, error) {
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

func (m *manager) CompileProject(ctx context.Context, request *types.CompileProjectRequest, response *types.CompileProjectResponse) error {
	return m.cm.Compile(ctx, request, response)
}
