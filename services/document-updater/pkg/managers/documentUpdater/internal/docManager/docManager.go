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
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"

	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater/internal/realTimeRedisManager"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater/internal/redisLocker"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater/internal/redisManager"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater/internal/trackChanges"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater/internal/updateManager"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater/internal/webApi"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/sharejs/types/text"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
)

type Manager interface {
	ClearProjectState(ctx context.Context, projectId primitive.ObjectID) error

	GetDoc(ctx context.Context, projectId, docId primitive.ObjectID) (*types.Doc, error)
	GetDocAndRecentUpdates(ctx context.Context, projectId, docId primitive.ObjectID, fromVersion sharedTypes.Version) (*types.Doc, []sharedTypes.DocumentUpdate, error)
	GetProjectDocsAndFlushIfOld(ctx context.Context, projectId primitive.ObjectID, newState string) ([]*types.Doc, error)

	SetDoc(ctx context.Context, projectId, docId primitive.ObjectID, request *types.SetDocRequest) error

	AddDoc(ctx context.Context, projectId primitive.ObjectID, update *types.AddDocUpdate) error
	AddFile(ctx context.Context, projectId primitive.ObjectID, update *types.AddFileUpdate) error
	RenameDoc(ctx context.Context, projectId primitive.ObjectID, update *types.RenameDocUpdate) error
	RenameFile(ctx context.Context, projectId primitive.ObjectID, update *types.RenameFileUpdate) error

	ProcessUpdatesForDocHeadless(ctx context.Context, projectId, docId primitive.ObjectID) error

	FlushDocIfLoaded(ctx context.Context, projectId, docId primitive.ObjectID) error
	FlushAndDeleteDoc(ctx context.Context, projectId, docId primitive.ObjectID) error
	FlushProject(ctx context.Context, projectId primitive.ObjectID) error
	FlushAndDeleteProject(ctx context.Context, projectId primitive.ObjectID) error
	QueueFlushAndDeleteProject(ctx context.Context, projectId primitive.ObjectID) error
}

func New(options *types.Options, client redis.UniversalClient, db *mongo.Database) (Manager, error) {
	rl, err := redisLocker.New(client)
	if err != nil {
		return nil, err
	}
	rm := redisManager.New(client)
	rtRm, err := realTimeRedisManager.New(client)
	if err != nil {
		return nil, err
	}
	web, err := webApi.New(options, db)
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

func (m *manager) AddDoc(ctx context.Context, projectId primitive.ObjectID, _ *types.AddDocUpdate) error {
	if err := m.rm.ClearProjectState(ctx, projectId); err != nil {
		return errors.Tag(
			err, "cannot clear project state ahead of adding doc",
		)
	}
	return nil
}

func (m *manager) AddFile(ctx context.Context, projectId primitive.ObjectID, _ *types.AddFileUpdate) error {
	if err := m.rm.ClearProjectState(ctx, projectId); err != nil {
		return errors.Tag(
			err, "cannot clear project state ahead of adding file",
		)
	}
	return nil
}

func (m *manager) RenameDoc(ctx context.Context, projectId primitive.ObjectID, update *types.RenameDocUpdate) error {
	docId := update.Id
	for {
		var err error
		if err = m.rm.ClearProjectState(ctx, projectId); err != nil {
			return errors.Tag(
				err, "cannot clear project state ahead of doc rename",
			)
		}
		lockErr := m.rl.RunWithLock(ctx, docId, func(ctx context.Context) {
			if _, err = m.rm.GetDocVersion(ctx, docId); err != nil {
				if errors.IsNotFoundError(err) {
					// Fast path: Doc is not loaded in redis yet.
					err = nil
				}
				return
			}

			var doc *types.Doc
			doc, err = m.processUpdatesForDoc(ctx, projectId, docId)
			if err != nil {
				return
			}
			err = m.rm.RenameDoc(ctx, projectId, docId, doc, update)
		})
		if err == errPartialFlush {
			continue
		}

		detachedCtx, done :=
			context.WithTimeout(context.Background(), 10*time.Second)
		err2 := m.rm.ClearProjectState(detachedCtx, projectId)
		done()

		if err != nil {
			return err
		}
		if err2 != nil {
			return errors.Tag(
				err2, "cannot clear project state after doc rename",
			)
		}
		if lockErr != nil {
			return lockErr
		}
		return nil
	}
}

func (m *manager) RenameFile(ctx context.Context, projectId primitive.ObjectID, _ *types.RenameFileUpdate) error {
	if err := m.rm.ClearProjectState(ctx, projectId); err != nil {
		return errors.Tag(
			err, "cannot clear project state ahead of renaming file",
		)
	}
	return nil
}

func (m *manager) ClearProjectState(ctx context.Context, projectId primitive.ObjectID) error {
	return m.rm.ClearProjectState(ctx, projectId)
}

func (m *manager) GetDocAndRecentUpdates(ctx context.Context, projectId, docId primitive.ObjectID, fromVersion sharedTypes.Version) (*types.Doc, []sharedTypes.DocumentUpdate, error) {
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
	if !errors.IsNotFoundError(err) {
		return nil, errors.Tag(err, "cannot get doc from redis")
	}
	flushedDoc, err := m.webApi.GetDoc(ctx, projectId, docId)
	if err != nil {
		return nil, errors.Tag(err, "cannot get doc from mongo")
	}
	doc = types.DocFromFlushedDoc(flushedDoc, projectId, docId)
	if err = m.rm.PutDocInMemory(ctx, projectId, docId, doc); err != nil {
		return nil, errors.Tag(err, "cannot put doc in memory")
	}
	return doc, nil
}

func (m *manager) SetDoc(ctx context.Context, projectId, docId primitive.ObjectID, request *types.SetDocRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	for {
		var err error
		lockErr := m.rl.RunWithLock(ctx, docId, func(ctx context.Context) {
			var doc *types.Doc
			doc, err = m.processUpdatesForDoc(ctx, projectId, docId)
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

				now := time.Now()
				updates := []sharedTypes.DocumentUpdate{{
					Version: doc.Version,
					DocId:   docId,
					Hash:    request.Snapshot().Hash(),
					Op:      op,
					Meta: sharedTypes.DocumentUpdateMeta{
						Type:          "external",
						Source:        sharedTypes.PublicId(request.Source),
						UserId:        request.UserId,
						IngestionTime: &now,
					},
				}}

				initialVersion := doc.Version
				updates, _, err = m.u.ProcessUpdates(
					ctx, docId, doc, updates, nil,
				)
				if err != nil {
					return
				}

				err = m.persistProcessedUpdates(
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

func (m *manager) ProcessUpdatesForDocHeadless(ctx context.Context, projectId, docId primitive.ObjectID) error {
	_, err := m.processUpdatesForDocAndMaybeFlushOld(ctx, projectId, docId, false)
	if err != nil && !errors.IsAlreadyReported(err) {
		m.reportError(projectId, docId, err)
		err = errors.MarkAsReported(err)
	}
	return err
}

const (
	maxUnFlushedAge = 5 * time.Minute
)

func (m *manager) processUpdatesForDocAndMaybeFlushOld(ctx context.Context, projectId, docId primitive.ObjectID, flushOld bool) (*types.Doc, error) {
	var doc *types.Doc
	var err error

	for {
		lockErr := m.rl.TryRunWithLock(ctx, docId, func(ctx context.Context) {
			doc, err = m.processUpdatesForDoc(ctx, projectId, docId)
			if err != nil {
				return
			}
			if flushOld && doc.UnFlushedTime != 0 {
				maxAge := types.UnFlushedTime(
					time.Now().Add(-maxUnFlushedAge).Unix(),
				)
				if doc.UnFlushedTime < maxAge {
					err = m.doFlushAndMaybeDelete(
						ctx, projectId, docId, doc, false,
					)
				}
			}
		})
		if lockErr == redisLocker.ErrLocked {
			return nil, nil
		}
		if err == errPartialFlush {
			err = nil
			continue
		}
		if err != nil {
			return nil, err
		}
		if lockErr != nil {
			return nil, lockErr
		}
		return doc, nil
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

func (m *manager) processUpdatesForDoc(ctx context.Context, projectId, docId primitive.ObjectID) (*types.Doc, error) {
	doc, err := m.getDoc(ctx, projectId, docId)
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		// Processing timed out.
		return nil, err
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

	transformUpdatesCache := make([]sharedTypes.DocumentUpdate, 0)
	var processed []sharedTypes.DocumentUpdate
	var updateErr error

	for time.Now().Before(softDeadline) {
		if err = ctx.Err(); err != nil {
			// Processing timed out.
			return nil, err
		}
		processed, transformUpdatesCache, updateErr =
			m.u.ProcessOutstandingUpdates(
				ctx, docId, doc, transformUpdatesCache,
			)

		if err = ctx.Err(); err != nil {
			// Processing timed out.
			return nil, err
		}
		if len(processed) == 0 && updateErr == nil {
			return doc, nil
		}

		err = m.persistProcessedUpdates(
			ctx,
			projectId, docId,
			doc, initialVersion,
			processed, updateErr,
		)
		if err != nil {
			return nil, err
		}

		if n := len(transformUpdatesCache); n > maxCacheSize {
			transformUpdatesCache = transformUpdatesCache[n-maxCacheSize:]
		}
	}
	return nil, errPartialFlush
}

func (m *manager) persistProcessedUpdates(
	ctx context.Context,
	projectId, docId primitive.ObjectID,
	doc *types.Doc,
	initialVersion sharedTypes.Version,
	processed []sharedTypes.DocumentUpdate,
	updateErr error,
) error {
	var queueDepth int64
	var err error
	appliedUpdates := make([]sharedTypes.DocumentUpdate, 0, len(processed))
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
			return err
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
		m.reportError(projectId, docId, updateErr)
		return errors.MarkAsReported(updateErr)
	}

	if len(appliedUpdates) != 0 {
		err = m.tc.RecordAndFlushHistoryOps(
			ctx, projectId, docId, int64(len(appliedUpdates)), queueDepth,
		)
		if err != nil {
			return errors.Tag(err, "cannot record and flush history")
		}
	}
	return nil
}

func (m *manager) reportError(projectId, docId primitive.ObjectID, err error) {
	// NOTE: This used to be in the background in Node.JS.
	//       Move in foreground to avoid race-conditions.
	reportCtx, cancel := context.WithTimeout(
		context.Background(), time.Second*10,
	)
	err2 := m.rtRm.ReportError(reportCtx, docId, err)
	cancel()
	if err2 != nil {
		ids := projectId.Hex() + "/" + docId.Hex()
		err2 = errors.Tag(err2, "cannot report error in "+ids)
		log.Println(err2.Error())
	}
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
				if delete {
					m.tc.FlushDocInBackground(projectId, docId)
				}
				return
			}
			var doc *types.Doc
			doc, err = m.processUpdatesForDoc(ctx, projectId, docId)
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
	if deleteFromRedis {
		m.tc.FlushDocInBackground(projectId, docId)
	}
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

func (m *manager) GetProjectDocsAndFlushIfOld(ctx context.Context, projectId primitive.ObjectID, newState string) ([]*types.Doc, error) {
	eg, pCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return m.rm.CheckOrSetProjectState(pCtx, projectId, newState)
	})
	var docIds []primitive.ObjectID
	eg.Go(func() error {
		var err error
		docIds, err = m.rm.GetDocIdsInProject(ctx, projectId)
		return err
	})
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	docs := make([]*types.Doc, len(docIds))
	eg, pCtx = errgroup.WithContext(ctx)
	for j, id := range docIds {
		i := j
		docId := id
		eg.Go(func() error {
			doc, err := m.processUpdatesForDocAndMaybeFlushOld(
				pCtx, projectId, docId, true,
			)
			if err != nil {
				return errors.Tag(err, projectId.Hex()+"/"+docId.Hex())
			}
			docs[i] = doc
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return docs, nil
}
