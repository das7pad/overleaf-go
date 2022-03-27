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

package webApi

import (
	"context"

	"github.com/edgedb/edgedb-go"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/pkg/models/doc"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/managers/docstore"

	"github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
)

type Manager interface {
	GetDoc(ctx context.Context, projectId, docId edgedb.UUID) (*types.FlushedDoc, error)
	SetDoc(ctx context.Context, projectId, docId edgedb.UUID, doc *doc.ForDocUpdate) error
}

func New(options *types.Options, c *edgedb.Client, db *mongo.Database) (Manager, error) {
	dm, err := docstore.New(options.APIs.Docstore.Options, c, db)
	if err != nil {
		return nil, err
	}
	pm := project.New(c, db)
	return &monolithManager{dm: dm, pm: pm}, nil
}
