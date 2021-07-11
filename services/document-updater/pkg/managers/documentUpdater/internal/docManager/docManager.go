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
	"github.com/das7pad/document-updater/pkg/managers/documentUpdater/internal/trackChanges"
	"github.com/das7pad/document-updater/pkg/managers/documentUpdater/internal/updateManager"
	"github.com/das7pad/document-updater/pkg/managers/documentUpdater/internal/webApi"
	"github.com/das7pad/document-updater/pkg/sharejs/types/text"
	"github.com/das7pad/document-updater/pkg/types"
)

type Manager interface {
	ClearProjectState(ctx context.Context, projectId primitive.ObjectID) error

	GetDoc(ctx context.Context, projectId, docId primitive.ObjectID) (*types.Doc, error)
	GetDocAndRecentUpdates(ctx context.Context, projectId, docId primitive.ObjectID, fromVersion types.Version) (*types.Doc, []types.DocumentUpdate, error)
	GetProjectDocsAndFlushIfOld(ctx context.Context, projectId primitive.ObjectID, newState string) ([]*types.DocContent, error)

	SetDoc(ctx context.Context, projectId, docId primitive.ObjectID, request *types.SetDocRequest) error

	ProcessUpdatesForDoc(ctx context.Context, projectId, docId primitive.ObjectID) (*types.Doc, int64, error)

	FlushDocIfLoaded(ctx context.Context, projectId, docId primitive.ObjectID) error
	FlushAndDeleteDoc(ctx context.Context, projectId, docId primitive.ObjectID) error
	FlushProject(ctx context.Context, projectId primitive.ObjectID) error
	FlushAndDeleteProject(ctx context.Context, projectId primitive.ObjectID) error
	QueueFlushAndDeleteProject(ctx context.Context, projectId primitive.ObjectID) error
}

func New(options *types.Options, client redis.UniversalClient) (Manager, error) {
	rl := redisLocker.New(client)
	rm := redisManager.New(client)
	rtRm := realTimeRedisManager.New(client)
	web, err := webApi.New(options)
	if err != nil {
		return nil, err
	}
	tc, err := trackChanges.New(options, client)
	if err != nil {
		return nil, err
	}
	u := updateManager.New(rm, rtRm)
	return &manager{
		rl:     rl,
		rm:     rm,
		rtRm:   rtRm,
		tc:     tc,
		u:      u,
		webApi: web,
	}, nil
}

type manager struct {
	rl     redisLocker.Locker
	rm     redisManager.Manager
	rtRm   realTimeRedisManager.Manager
	tc     trackChanges.Manager
	u      updateManager.Manager
	webApi webApi.Manager
}

func (m *manager) ClearProjectState(ctx context.Context, projectId primitive.ObjectID) error {
	return m.rm.ClearProjectState(ctx, projectId)
}

func (m *manager) GetDocAndRecentUpdates(ctx context.Context, projectId, docId primitive.ObjectID, fromVersion types.Version) (*types.Doc, []types.DocumentUpdate, error) {
	doc, err := m.GetDoc(ctx, projectId, docId)
	if err != nil {
		return nil, nil, err
	}
	updates, err :=
		m.rm.GetPreviousDocUpdates(ctx, docId, fromVersion, doc.Version)
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
	if err != nil {
		return nil, errors.Tag(err, "cannot get doc from mongo")
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

func (m *manager) SetDoc(ctx context.Context, projectId, docId primitive.ObjectID, request *types.SetDocRequest) error {
	for {
		var err error
		lockErr := m.rl.RunWithLock(ctx, docId, func(ctx context.Context) {
			var doc *types.Doc
			doc, _, err = m.processUpdatesForDoc(ctx, projectId, docId)
			if err != nil {
				return
			}

			if err = ctx.Err(); err != nil {
				// Processing timed out.
				return
			}

			op := text.Diff(doc.Snapshot, request.Snapshot())

			if err = ctx.Err(); err != nil {
				// Processing timed out.
				return
			}

			if len(op) > 0 {
				if request.Undoing {
					for i := range op {
						op[i].Undo = true
					}
				}

				updates := []types.DocumentUpdate{{
					Version: doc.Version,
					DocId:   docId,
					Hash:    request.Snapshot().Hash(),
					Op:      op,
					Meta: types.DocumentUpdateMeta{
						Type:   "external",
						Source: types.PublicId(request.Source),
						UserId: request.UserId,
					},
				}}

				initialVersion := doc.Version
				updates, _, err = m.u.ProcessUpdates(
					ctx, docId, doc, updates, nil,
				)
				if err != nil {
					return
				}

				_, err = m.persistProcessedUpdates(
					ctx,
					projectId, docId,
					doc, initialVersion,
					updates, nil,
				)
				if err != nil {
					return
				}
			}

			deleteFromRedis := doc.JustLoadedIntoRedis
			err = m.doFlushAndMaybeDelete(
				ctx, projectId, docId, doc, deleteFromRedis,
			)
			if err != nil {
				return
			}
		})
		if err == errPartialFlush {
			continue
		}
		if err != nil {
			return err
		}
		if lockErr != nil {
			return lockErr
		}
		return nil
	}
}

func (m *manager) ProcessUpdatesForDoc(ctx context.Context, projectId, docId primitive.ObjectID) (*types.Doc, int64, error) {
	var doc *types.Doc
	var err error
	queueDepth := int64(-1)

	for {
		lockErr := m.rl.TryRunWithLock(ctx, docId, func(ctx context.Context) {
			doc, queueDepth, err = m.processUpdatesForDoc(ctx, projectId, docId)
		})
		if lockErr == redisLocker.ErrLocked {
			return nil, queueDepth, nil
		}
		if err == errPartialFlush {
			err = nil
			continue
		}
		if err != nil {
			return nil, queueDepth, err
		}
		if lockErr != nil {
			return nil, queueDepth, lockErr
		}
		return doc, queueDepth, nil
	}
}

const (
	lockUsageProcessOutstandingUpdates = 0.5
	maxBudgetProcessOutstandingUpdates = 10 * time.Second
	maxCacheSize                       = 100
)

var (
	errPartialFlush = errors.New("partial flush")
)

func (m *manager) processUpdatesForDoc(ctx context.Context, projectId, docId primitive.ObjectID) (*types.Doc, int64, error) {
	doc, err := m.getDoc(ctx, projectId, docId)
	if err != nil {
		return nil, 0, err
	}
	if err = ctx.Err(); err != nil {
		// Processing timed out.
		return nil, 0, err
	}
	initialVersion := doc.Version
	now := time.Now()
	softDeadline := now.Add(maxBudgetProcessOutstandingUpdates)
	if hardDeadline, hasDeadline := ctx.Deadline(); hasDeadline {
		dynamicBudget := time.Duration(
			float64(hardDeadline.Sub(now)) *
				lockUsageProcessOutstandingUpdates,
		)
		if dynamicBudget < maxBudgetProcessOutstandingUpdates {
			softDeadline = now.Add(dynamicBudget)
		}
	}

	transformUpdatesCache := make([]types.DocumentUpdate, 0)
	var processed []types.DocumentUpdate
	var updateErr error
	queueDepth := int64(-1)

	for time.Now().Before(softDeadline) {
		if err = ctx.Err(); err != nil {
			// Processing timed out.
			return nil, 0, err
		}
		processed, transformUpdatesCache, updateErr =
			m.u.ProcessOutstandingUpdates(
				ctx, docId, doc, transformUpdatesCache,
			)

		if err = ctx.Err(); err != nil {
			// Processing timed out.
			return nil, 0, err
		}
		if len(processed) == 0 && updateErr == nil {
			return doc, queueDepth, nil
		}

		queueDepth, err = m.persistProcessedUpdates(
			ctx,
			projectId, docId,
			doc, initialVersion,
			processed, updateErr,
		)
		if err != nil {
			return nil, 0, err
		}

		if n := len(transformUpdatesCache); n > maxCacheSize {
			transformUpdatesCache = transformUpdatesCache[n-maxCacheSize:]
		}
	}
	return nil, 0, errPartialFlush
}

func (m *manager) persistProcessedUpdates(
	ctx context.Context,
	projectId, docId primitive.ObjectID,
	doc *types.Doc,
	initialVersion types.Version,
	processed []types.DocumentUpdate,
	updateErr error,
) (int64, error) {
	var queueDepth int64
	var err error
	appliedUpdates := make([]types.DocumentUpdate, 0, len(processed))
	if doc.Version != initialVersion {
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
			return queueDepth, err
		}
	}

	if len(processed) != 0 {
		// NOTE: This used to be in the background in Node.JS.
		//       Move in foreground to avoid race-conditions.
		confirmCtx, cancel := context.WithTimeout(
			context.Background(), time.Second*10,
		)
		err = m.rtRm.ConfirmUpdates(confirmCtx, processed)
		cancel()
		if err != nil {
			ids := projectId.Hex() + "/" + docId.Hex()
			err = errors.Tag(err, "cannot confirm updates in "+ids)
			log.Println(err.Error())
		}
	}

	if updateErr != nil {
		// NOTE: This used to be in the background in Node.JS.
		//       Move in foreground to avoid race-conditions.
		reportCtx, cancel := context.WithTimeout(
			context.Background(), time.Second*10,
		)
		err = m.rtRm.ReportError(reportCtx, docId, updateErr)
		cancel()
		if err != nil {
			ids := projectId.Hex() + "/" + docId.Hex()
			err = errors.Tag(err, "cannot report error in "+ids)
			log.Println(err.Error())
		}
		return 0, updateErr
	}

	if len(appliedUpdates) != 0 {
		err = m.tc.RecordAndFlushHistoryOps(
			ctx, projectId, docId, int64(len(appliedUpdates)), queueDepth,
		)
		if err != nil {
			return 0, errors.Tag(err, "cannot record and flush history")
		}
	}
	return queueDepth, nil
}

func (m *manager) tryCheckDocNotLoadedOrFlushed(ctx context.Context, docId primitive.ObjectID) bool {
	// Look for the doc (version) and updates gracefully, ignoring errors.
	// It's only used for taking a short-cut when already flushed.
	// Else we go the long way of fetch doc, check for updates and then flush.
	// Not taking the fast path is OK.
	_, err := m.rm.GetDocVersion(ctx, docId)
	if err == nil || errors.GetCause(err) != redis.Nil {
		return false
	}
	n, err := m.rtRm.GetUpdatesLength(ctx, docId)
	if err != nil {
		return false
	}
	return n == 0
}

func (m *manager) FlushDocIfLoaded(ctx context.Context, projectId, docId primitive.ObjectID) error {
	return m.flushAndMaybeDeleteDoc(ctx, projectId, docId, false)
}

func (m *manager) FlushAndDeleteDoc(ctx context.Context, projectId, docId primitive.ObjectID) error {
	return m.flushAndMaybeDeleteDoc(ctx, projectId, docId, true)
}

func (m *manager) flushAndMaybeDeleteDoc(ctx context.Context, projectId, docId primitive.ObjectID, delete bool) error {
	var err error

	for {
		lockErr := m.rl.RunWithLock(ctx, docId, func(ctx context.Context) {
			if m.tryCheckDocNotLoadedOrFlushed(ctx, docId) {
				return
			}
			var doc *types.Doc
			doc, _, err = m.processUpdatesForDoc(ctx, projectId, docId)
			if err != nil {
				return
			}
			err = m.doFlushAndMaybeDelete(
				ctx, projectId, docId, doc, delete,
			)
			if err != nil {
				return
			}
		})
		if err == errPartialFlush {
			err = nil
			continue
		}
		if err != nil {
			return err
		}
		if lockErr != nil {
			return lockErr
		}
		return nil
	}
}

func (m *manager) doFlushAndMaybeDelete(ctx context.Context, projectId, docId primitive.ObjectID, doc *types.Doc, deleteFromRedis bool) error {
	if doc.UnFlushedTime != 0 {
		err := m.webApi.SetDoc(
			ctx, projectId, docId, doc.ToSetDocDetails(),
		)
		if err != nil {
			return err
		}
	}
	if deleteFromRedis {
		err := m.rm.RemoveDocFromMemory(ctx, projectId, docId)
		if err != nil {
			return err
		}
	} else if doc.UnFlushedTime != 0 {
		err := m.rm.ClearUnFlushedTime(ctx, docId)
		if err != nil {
			return err
		}
	}
	doc.UnFlushedTime = 0
	return nil
}

func (m *manager) FlushProject(ctx context.Context, projectId primitive.ObjectID) error {
	return m.operateOnAllProjectDocs(ctx, projectId, m.FlushDocIfLoaded)
}

func (m *manager) FlushAndDeleteProject(ctx context.Context, projectId primitive.ObjectID) error {
	return m.operateOnAllProjectDocs(ctx, projectId, m.FlushAndDeleteDoc)
}

func (m *manager) QueueFlushAndDeleteProject(ctx context.Context, projectId primitive.ObjectID) error {
	return m.rm.QueueFlushAndDeleteProject(ctx, projectId)
}

type projectDocOperation func(ctx context.Context, projectId, docId primitive.ObjectID) error

func (m *manager) operateOnAllProjectDocs(ctx context.Context, projectId primitive.ObjectID, operation projectDocOperation) error {
	docIds, err := m.rm.GetDocIdsInProject(ctx, projectId)
	if err != nil {
		return err
	}
	errs := make([]error, 0)
	for _, docId := range docIds {
		err2 := operation(ctx, projectId, docId)
		if err2 != nil {
			ids := projectId.Hex() + "/" + docId.Hex()
			errs = append(errs, errors.Tag(err2, ids))
		}
	}
	if len(errs) != 0 {
		s := ""
		for i, err2 := range errs {
			if i > 0 {
				s += " | "
			}
			s += err2.Error()
		}
		return errors.New(s)
	}
	return nil
}

func (m *manager) GetProjectDocsAndFlushIfOld(ctx context.Context, projectId primitive.ObjectID, newState string) ([]*types.DocContent, error) {
	err := m.rm.CheckOrSetProjectState(ctx, projectId, newState)
	if err != nil {
		return nil, err
	}
	docIds, err := m.rm.GetDocIdsInProject(ctx, projectId)
	if err != nil {
		return nil, err
	}
	docs := make([]*types.DocContent, len(docIds))
	for i, docId := range docIds {
		// TODO: force flush for old docs
		doc, _, err2 := m.ProcessUpdatesForDoc(ctx, projectId, docId)
		if err2 != nil {
			return nil, errors.Tag(err2, projectId.Hex()+"/"+docId.Hex())
		}
		docs[i] = doc.ToDocContent(docId)
	}
	return docs, nil
}
