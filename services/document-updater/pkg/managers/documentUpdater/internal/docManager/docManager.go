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

package docManager

import (
	"context"
	"log"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/document-updater/pkg/errors"
	"github.com/das7pad/document-updater/pkg/managers/documentUpdater/internal/redisLocker"
	"github.com/das7pad/document-updater/pkg/managers/documentUpdater/internal/redisManager"
	"github.com/das7pad/document-updater/pkg/managers/documentUpdater/internal/webApi"
	"github.com/das7pad/document-updater/pkg/types"
)

type Manager interface {
	GetDoc(ctx context.Context, projectId, docId primitive.ObjectID) (*types.Doc, error)
	GetDocAndRecentUpdates(ctx context.Context, projectId, docId primitive.ObjectID, fromVersion types.Version) (*types.Doc, []types.DocumentUpdate, error)
}

func New(options *types.Options, client redis.UniversalClient) (Manager, error) {
	rl := redisLocker.New(client)
	rm := redisManager.New(client)
	web, err := webApi.New(options)
	if err != nil {
		return nil, err
	}
	return &manager{
		rl:     rl,
		rm:     rm,
		webApi: web,
	}, nil
}

type manager struct {
	rl     redisLocker.Locker
	rm     redisManager.Manager
	webApi webApi.Manager
}

func (m *manager) GetDocAndRecentUpdates(ctx context.Context, projectId, docId primitive.ObjectID, fromVersion types.Version) (*types.Doc, []types.DocumentUpdate, error) {
	doc, err := m.GetDoc(ctx, projectId, docId)
	if err != nil {
		return nil, nil, err
	}
	updates, err := m.rm.GetPreviousDocOps(ctx, docId, fromVersion, doc.Version)
	if err != nil {
		return nil, nil, err
	}
	return doc, updates, nil
}

func (m *manager) GetDoc(ctx context.Context, projectId, docId primitive.ObjectID) (*types.Doc, error) {
	doc, err := m.rm.GetDoc(ctx, projectId, docId)
	if err == nil {
		return doc, nil
	}
	if !errors.IsNotFoundError(err) {
		return nil, err
	}
	err = nil
	lockErr := m.rl.RunWithLock(ctx, docId, func(ctx context.Context) {
		doc, err = m.rm.GetDoc(ctx, projectId, docId)
		if err == nil {
			return
		}
		err = errors.Tag(err, "cannot get doc from redis")
		if !errors.IsNotFoundError(err) {
			return
		}
		var flushedDoc *types.FlushedDoc
		flushedDoc, err = m.webApi.GetDoc(ctx, projectId, docId)
		if !errors.IsNotFoundError(err) {
			return
		}
		doc = flushedDoc.ToDoc(projectId)
		err = m.rm.PutDocInMemory(ctx, projectId, docId, doc)
		if err != nil {
			ids := projectId.Hex() + "/" + docId.Hex()
			log.Println(
				errors.Tag(
					err, "cannot put doc in memory ("+ids+")",
				).Error(),
			)
			// do not fail the request
			err = nil
			return
		}
	})
	if err != nil {
		return nil, err
	}
	if lockErr != nil {
		return nil, err
	}
	return doc, nil
}
