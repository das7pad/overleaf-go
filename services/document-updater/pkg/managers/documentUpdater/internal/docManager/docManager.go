// Golang port of Overleaf
// Copyright (C) 2021-2023 Jakob Ackermann <das7pad@outlook.com>
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
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/doc"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/redisLocker"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater/internal/realTimeRedisManager"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater/internal/redisManager"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater/internal/trackChanges"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater/internal/updateManager"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/sharejs/types/text"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
)

type Manager interface {
	GetDoc(ctx context.Context, projectId, docId sharedTypes.UUID) (*types.Doc, error)
	GetDocAndRecentUpdates(ctx context.Context, projectId, docId sharedTypes.UUID, fromVersion sharedTypes.Version) (*types.Doc, []sharedTypes.DocumentUpdate, error)
	GetProjectDocsAndFlushIfOld(ctx context.Context, projectId sharedTypes.UUID) ([]*types.Doc, error)
	SetDoc(ctx context.Context, projectId, docId sharedTypes.UUID, request types.SetDocRequest) error
	RenameDoc(ctx context.Context, projectId, docId sharedTypes.UUID, newPath sharedTypes.PathName) error
	ProcessUpdatesForDocHeadless(ctx context.Context, projectId, docId sharedTypes.UUID) error
	FlushAndDeleteDoc(ctx context.Context, projectId, docId sharedTypes.UUID) error
	FlushProject(ctx context.Context, projectId sharedTypes.UUID) error
	FlushAndDeleteProject(ctx context.Context, projectId sharedTypes.UUID) error
	QueueFlushAndDeleteProject(ctx context.Context, projectId sharedTypes.UUID) error
}

func New(db *pgxpool.Pool, client redis.UniversalClient) (Manager, error) {
	rl, err := redisLocker.New(client, "Blocking")
	if err != nil {
		return nil, err
	}
	rm := redisManager.New(client)
	rtRm, err := realTimeRedisManager.New(client)
	if err != nil {
		return nil, err
	}
	tc, err := trackChanges.New(db, client)
	if err != nil {
		return nil, err
	}
	u := updateManager.New(rm, rtRm)
	return &manager{
		rl:   rl,
		rm:   rm,
		rtRm: rtRm,
		tc:   tc,
		u:    u,
		dm:   doc.New(db),
		pm:   project.New(db),
	}, nil
}

type manager struct {
	rl   redisLocker.Locker
	rm   redisManager.Manager
	rtRm realTimeRedisManager.Manager
	tc   trackChanges.Manager
	u    updateManager.Manager
	dm   doc.Manager
	pm   project.Manager
}

func (m *manager) RenameDoc(ctx context.Context, projectId, docId sharedTypes.UUID, newPath sharedTypes.PathName) error {
	for {
		err := m.rl.RunWithLock(ctx, docId, func(ctx context.Context) error {
			if _, err := m.rm.GetDocVersion(ctx, docId); err != nil {
				if errors.IsNotFoundError(err) {
					// Fast path: Doc is not loaded in redis yet.
					return nil
				}
				return err
			}

			d, err := m.processUpdatesForDoc(ctx, projectId, docId)
			if err != nil {
				return err
			}
			return m.rm.RenameDoc(ctx, projectId, docId, d, newPath)
		})
		if err == errPartialFlush {
			continue
		}
		if err != nil {
			return err
		}
		return nil
	}
}

func (m *manager) GetDocAndRecentUpdates(ctx context.Context, projectId, docId sharedTypes.UUID, fromVersion sharedTypes.Version) (*types.Doc, []sharedTypes.DocumentUpdate, error) {
	d, err := m.GetDoc(ctx, projectId, docId)
	if err != nil {
		return nil, nil, err
	}
	updates, err := m.rm.GetPreviousDocUpdates(
		ctx, docId, fromVersion, d.Version,
	)
	if err != nil {
		return nil, nil, err
	}
	return d, updates, nil
}

func (m *manager) GetDoc(ctx context.Context, projectId, docId sharedTypes.UUID) (*types.Doc, error) {
	d, err := m.rm.GetDoc(ctx, projectId, docId)
	if err == nil {
		return d, nil
	}
	if !errors.IsNotFoundError(err) {
		return nil, err
	}
	err = m.rl.RunWithLock(ctx, docId, func(ctx context.Context) error {
		d, err = m.getDoc(ctx, projectId, docId)
		return err
	})
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (m *manager) getDoc(ctx context.Context, projectId, docId sharedTypes.UUID) (*types.Doc, error) {
	d, err := m.rm.GetDoc(ctx, projectId, docId)
	if err == nil {
		return d, nil
	}
	if !errors.IsNotFoundError(err) {
		return nil, errors.Tag(err, "cannot get doc from redis")
	}
	flushedDoc, err := m.pm.GetDoc(ctx, projectId, docId)
	if err != nil {
		return nil, errors.Tag(err, "cannot get doc from db")
	}
	d = types.DocFromFlushedDoc(flushedDoc, projectId, docId)
	if err = m.rm.PutDocInMemory(ctx, projectId, docId, d); err != nil {
		return nil, errors.Tag(err, "cannot put doc in memory")
	}
	return d, nil
}

func (m *manager) SetDoc(ctx context.Context, projectId, docId sharedTypes.UUID, request types.SetDocRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	for {
		err := m.rl.RunWithLock(ctx, docId, func(ctx context.Context) error {
			d, err := m.processUpdatesForDoc(ctx, projectId, docId)
			if err != nil {
				return err
			}

			if err = ctx.Err(); err != nil {
				// Processing timed out.
				return err
			}

			op := text.Diff(d.Snapshot, request.Snapshot)

			if err = ctx.Err(); err != nil {
				// Processing timed out.
				return err
			}

			if len(op) > 0 {
				now := time.Now()
				updates := []sharedTypes.DocumentUpdate{{
					Version: d.Version,
					DocId:   docId,
					Hash:    request.Snapshot.Hash(),
					Op:      op,
					Meta: sharedTypes.DocumentUpdateMeta{
						Type:          "external",
						Source:        sharedTypes.PublicId(request.Source),
						UserId:        request.UserId,
						IngestionTime: &now,
					},
				}}

				initialVersion := d.Version
				updates, _, err = m.u.ProcessUpdates(
					ctx, docId, d, updates, nil,
				)
				if err != nil {
					return err
				}

				err = m.persistProcessedUpdates(
					ctx,
					projectId, docId,
					d, initialVersion,
					updates, nil,
				)
				if err != nil {
					return err
				}
			}

			deleteFromRedis := d.JustLoadedIntoRedis
			err = m.doFlushAndMaybeDelete(
				ctx, projectId, docId, d, deleteFromRedis,
			)
			if err != nil {
				return err
			}
			return nil
		})
		if err == errPartialFlush {
			continue
		}
		if err != nil {
			return err
		}
		return nil
	}
}

func (m *manager) ProcessUpdatesForDocHeadless(ctx context.Context, projectId, docId sharedTypes.UUID) error {
	for {
		err := m.rl.TryRunWithLock(ctx, docId, func(ctx context.Context) error {
			_, err := m.processUpdatesForDoc(ctx, projectId, docId)
			return err
		})
		if err == redisLocker.ErrLocked {
			// Someone else is processing updates already.
			return nil
		}
		if err == errPartialFlush {
			continue
		}
		if err != nil && !errors.IsAlreadyReported(err) {
			m.reportError(projectId, docId, err)
			err = errors.MarkAsReported(err)
		}
		return err
	}
}

const (
	maxUnFlushedAge = 5 * time.Minute
)

func (m *manager) processUpdatesForDocAndFlushOld(ctx context.Context, projectId, docId sharedTypes.UUID) (*types.Doc, error) {
	var d *types.Doc

	for {
		err := m.rl.RunWithLock(ctx, docId, func(ctx context.Context) error {
			var err error
			d, err = m.processUpdatesForDoc(ctx, projectId, docId)
			if err != nil {
				return err
			}
			if d.UnFlushedTime != 0 {
				maxAge := types.UnFlushedTime(
					time.Now().Add(-maxUnFlushedAge).Unix(),
				)
				if d.UnFlushedTime < maxAge {
					err = m.doFlushAndMaybeDelete(
						ctx, projectId, docId, d, false,
					)
					if err != nil {
						return err
					}
				}
			}
			return nil
		})
		if err == errPartialFlush {
			continue
		}
		if err != nil {
			return nil, err
		}
		return d, nil
	}
}

const (
	lockUsageProcessOutstandingUpdates = 0.5
	maxBudgetProcessOutstandingUpdates = 10 * time.Second
	maxCacheSize                       = 100
)

var errPartialFlush = errors.New("partial flush")

func (m *manager) processUpdatesForDoc(ctx context.Context, projectId, docId sharedTypes.UUID) (*types.Doc, error) {
	d, err := m.getDoc(ctx, projectId, docId)
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		// Processing timed out.
		return nil, err
	}
	initialVersion := d.Version
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

	transformCache := make([]sharedTypes.DocumentUpdate, 0)
	var processed []sharedTypes.DocumentUpdate
	var updateErr error

	for time.Now().Before(softDeadline) {
		if err = ctx.Err(); err != nil {
			// Processing timed out.
			return nil, err
		}
		processed, transformCache, updateErr = m.u.ProcessOutstandingUpdates(
			ctx, docId, d, transformCache,
		)

		if err = ctx.Err(); err != nil {
			// Processing timed out.
			return nil, err
		}
		if len(processed) == 0 && updateErr == nil {
			return d, nil
		}

		err = m.persistProcessedUpdates(
			ctx,
			projectId, docId,
			d, initialVersion,
			processed, updateErr,
		)
		if err != nil {
			return nil, err
		}

		if n := len(transformCache); n > maxCacheSize {
			transformCache = transformCache[n-maxCacheSize:]
		}
	}
	return nil, errPartialFlush
}

func (m *manager) persistProcessedUpdates(
	ctx context.Context,
	projectId, docId sharedTypes.UUID,
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
			ids := projectId.String() + "/" + docId.String()
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

func (m *manager) reportError(projectId, docId sharedTypes.UUID, err error) {
	// NOTE: This used to be in the background in Node.JS.
	//       Move in foreground to avoid race-conditions.
	reportCtx, cancel := context.WithTimeout(
		context.Background(), time.Second*10,
	)
	err2 := m.rtRm.ReportError(reportCtx, docId, err)
	cancel()
	if err2 != nil {
		ids := projectId.String() + "/" + docId.String()
		err2 = errors.Tag(err2, "cannot report error in "+ids)
		log.Println(err2.Error())
	}
}

func (m *manager) tryCheckDocNotLoadedOrFlushed(ctx context.Context, docId sharedTypes.UUID) bool {
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

func (m *manager) FlushDocIfLoaded(ctx context.Context, projectId, docId sharedTypes.UUID) error {
	return m.flushAndMaybeDeleteDoc(ctx, projectId, docId, false)
}

func (m *manager) FlushAndDeleteDoc(ctx context.Context, projectId, docId sharedTypes.UUID) error {
	return m.flushAndMaybeDeleteDoc(ctx, projectId, docId, true)
}

func (m *manager) flushAndMaybeDeleteDoc(ctx context.Context, projectId, docId sharedTypes.UUID, deleteFromRedis bool) error {
	for {
		err := m.rl.RunWithLock(ctx, docId, func(ctx context.Context) error {
			if m.tryCheckDocNotLoadedOrFlushed(ctx, docId) {
				if deleteFromRedis {
					m.tc.FlushDocInBackground(projectId, docId)
				}
				return nil
			}
			d, err := m.processUpdatesForDoc(ctx, projectId, docId)
			if err != nil {
				return err
			}
			err = m.doFlushAndMaybeDelete(
				ctx, projectId, docId, d, deleteFromRedis,
			)
			if err != nil {
				return err
			}
			return nil
		})
		if err == errPartialFlush {
			continue
		}
		if err != nil {
			return err
		}
		return nil
	}
}

func (m *manager) doFlushAndMaybeDelete(ctx context.Context, projectId, docId sharedTypes.UUID, doc *types.Doc, deleteFromRedis bool) error {
	if deleteFromRedis {
		m.tc.FlushDocInBackground(projectId, docId)
	}
	if doc.UnFlushedTime != 0 {
		err := m.dm.UpdateDoc(ctx, projectId, docId, doc.ToForDocUpdate())
		if err != nil {
			return errors.Tag(err, "cannot persist doc in db")
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

func (m *manager) FlushProject(ctx context.Context, projectId sharedTypes.UUID) error {
	return m.operateOnAllProjectDocs(ctx, projectId, m.FlushDocIfLoaded)
}

func (m *manager) FlushAndDeleteProject(ctx context.Context, projectId sharedTypes.UUID) error {
	return m.operateOnAllProjectDocs(ctx, projectId, m.FlushAndDeleteDoc)
}

func (m *manager) QueueFlushAndDeleteProject(ctx context.Context, projectId sharedTypes.UUID) error {
	return m.rm.QueueFlushAndDeleteProject(ctx, projectId)
}

type projectDocOperation func(ctx context.Context, projectId, docId sharedTypes.UUID) error

func (m *manager) operateOnAllProjectDocs(ctx context.Context, projectId sharedTypes.UUID, operation projectDocOperation) error {
	docIds, err := m.rm.GetDocIdsInProject(ctx, projectId)
	if err != nil {
		return err
	}
	errs := make([]error, 0)
	for _, docId := range docIds {
		err2 := operation(ctx, projectId, docId)
		if err2 != nil {
			ids := projectId.String() + "/" + docId.String()
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

func (m *manager) GetProjectDocsAndFlushIfOld(ctx context.Context, projectId sharedTypes.UUID) ([]*types.Doc, error) {
	docIds, errGetDocIds := m.rm.GetDocIdsInProject(ctx, projectId)
	if errGetDocIds != nil {
		return nil, errGetDocIds
	}

	docs := make([]*types.Doc, len(docIds))
	eg, pCtx := errgroup.WithContext(ctx)
	for j, id := range docIds {
		i := j
		docId := id
		eg.Go(func() error {
			d, err := m.processUpdatesForDocAndFlushOld(
				pCtx, projectId, docId,
			)
			if err != nil {
				return errors.Tag(err, projectId.String()+"/"+docId.String())
			}
			docs[i] = d
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return docs, nil
}
