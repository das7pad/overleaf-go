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

package documentUpdater

import (
	"context"

	"github.com/edgedb/edgedb-go"
	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	documentUpdaterTypes "github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

type Manager interface {
	GetDoc(ctx context.Context, projectId, docId edgedb.UUID, fromVersion sharedTypes.Version) (*documentUpdaterTypes.GetDocResponse, error)
	CheckDocExists(ctx context.Context, projectId, docId edgedb.UUID) error
	FlushProject(ctx context.Context, projectId edgedb.UUID) error
}

func New(options *types.Options, c *edgedb.Client, client redis.UniversalClient, db *mongo.Database) (Manager, error) {
	return documentUpdater.New(
		options.APIs.DocumentUpdater.Options,
		c,
		client,
		db,
	)
}
