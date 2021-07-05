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
	"time"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/document-updater/pkg/errors"
	"github.com/das7pad/document-updater/pkg/managers/documentUpdater/internal/realTimeRedisManager"
	"github.com/das7pad/document-updater/pkg/managers/documentUpdater/internal/redisLocker"
	"github.com/das7pad/document-updater/pkg/managers/documentUpdater/internal/redisManager"
	"github.com/das7pad/document-updater/pkg/managers/documentUpdater/internal/updateManager"
	"github.com/das7pad/document-updater/pkg/managers/documentUpdater/internal/webApi"
	"github.com/das7pad/document-updater/pkg/types"
)

type Manager interface {
	GetDoc(ctx context.Context, projectId, docId primitive.ObjectID) (*types.Doc, error)
	GetDocAndRecentUpdates(ctx context.Context, projectId, docId primitive.ObjectID, fromVersion types.Version) (*types.Doc, []types.DocumentUpdate, error)

	ProcessUpdatesForDoc(ctx context.Context, projectId, docId primitive.ObjectID) (*types.Doc, int64, error)
}

func New(options *types.Options, client redis.UniversalClient) (Manager, error) {
	rl := redisLocker.New(client)
	rm := redisManager.New(client)
	rtRm := realTimeRedisManager.New(client)
	web, err := webApi.New(options)
	if err != nil {
		return nil, err
	}
	u := updateManager.New(rm, rtRm)
	return &manager{
		rl:     rl,
		rm:     rm,
		rtRm:   rtRm,
		webApi: web,
		u:      u,
	}, nil
}

type manager struct {
	rl     redisLocker.Locker
	rm     redisManager.Manager
	rtRm   realTimeRedisManager.Manager
	u      updateManager.Manager
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
		doc, err = m.getDoc(ctx, projectId, docId)
	})
	if err != nil {
		return nil, err
	}
	if lockErr != nil {
		return nil, lockErr
	}
	return doc, nil
}
func (m *manager) getDoc(ctx context.Context, projectId, docId primitive.ObjectID) (*types.Doc, error) {
	doc, err := m.rm.GetDoc(ctx, projectId, docId)
	if err == nil {
		return doc, nil
	}
	err = errors.Tag(err, "cannot get doc from redis")
	if !errors.IsNotFoundError(err) {
		return nil, err
	}
	var flushedDoc *types.FlushedDoc
	flushedDoc, err = m.webApi.GetDoc(ctx, projectId, docId)
	if !errors.IsNotFoundError(err) {
		return nil, err
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
		return doc, nil
	}
	return doc, nil
}

func (m *manager) ProcessUpdatesForDoc(ctx context.Context, projectId, docId primitive.ObjectID) (*types.Doc, int64, error) {
	var doc *types.Doc
	var err error
	var queueDepth int64

	lockErr := m.rl.RunWithLock(ctx, docId, func(ctx context.Context) {
		doc, err = m.getDoc(ctx, projectId, docId)
		if err != nil {
			return
		}
		initialVersion := doc.Version

		processed, updateError := m.u.ProcessOutstandingUpdates(
			ctx, docId, doc,
		)

		if ctx.Err() != nil {
			// Processing timed out.
			err = ctx.Err()
			return
		}
		if doc.Version != initialVersion {
			appliedUpdates := make([]types.DocumentUpdate, 0, len(processed))
			for _, update := range processed {
				if update.Dup {
					continue
				}
				appliedUpdates = append(appliedUpdates, update)
			}
			queueDepth, err = m.rm.UpdateDocument(
				ctx, docId, doc, appliedUpdates,
			)
			if err != nil {
				return
			}
		}

		if len(processed) != 0 {
			// NOTE: This used to be in the background in Node.JS.
			//       Move in foreground to avoid race-conditions.
			confirmCtx, cancel := context.WithTimeout(
				context.Background(), time.Second*10,
			)
			defer cancel()
			err2 := m.rtRm.ConfirmUpdates(confirmCtx, processed)
			if err2 != nil {
				ids := projectId.Hex() + "/" + docId.Hex()
				err2 = errors.Tag(err2, "cannot confirm updates in "+ids)
				log.Println(err2.Error())
			}
		}

		if updateError != nil {
			err2 := m.rtRm.ReportError(ctx, docId, updateError)
			if err2 != nil {
				ids := projectId.Hex() + "/" + docId.Hex()
				err2 = errors.Tag(err2, "cannot report error in "+ids)
				log.Println(err2.Error())
			}
		}
	})
	if err != nil {
		return nil, queueDepth, err
	}
	if lockErr != nil {
		return nil, queueDepth, lockErr
	}
	return doc, queueDepth, nil
}
