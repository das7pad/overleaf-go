// Golang port of the Overleaf document-updater service
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

package historyRedisManager

import (
	"context"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/services/document-updater/pkg/errors"
)

type Manager interface {
	RecordDocHasHistory(ctx context.Context, projectId, docId primitive.ObjectID) error
}

func New(client redis.UniversalClient) Manager {
	return &manager{client: client}
}

type manager struct {
	client redis.UniversalClient
}

func getDocsWithHistoryOpsKey(projectId primitive.ObjectID) string {
	return "DocsWithHistoryOps:{" + projectId.Hex() + "}"
}

func (m *manager) RecordDocHasHistory(ctx context.Context, projectId, docId primitive.ObjectID) error {
	key := getDocsWithHistoryOpsKey(projectId)
	err := m.client.SAdd(ctx, key, docId.Hex()).Err()
	if err != nil {
		return errors.Tag(err, "cannot record doc has history")
	}
	return err
}
