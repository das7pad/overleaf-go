// Golang port of the Overleaf docstore service
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

package docArchive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/backend"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/managers/docstore/internal/docs"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/models"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/types"
)

type Manager interface {
	ArchiveDocs(
		ctx context.Context,
		projectId primitive.ObjectID,
	) error

	ArchiveDoc(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
	) error

	UnArchiveDocs(
		ctx context.Context,
		projectId primitive.ObjectID,
	) error

	UnArchiveDoc(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
	) error

	DestroyDocs(
		ctx context.Context,
		projectId primitive.ObjectID,
	) error
}

func New(backendOptions backend.Options, bucket string, pLimits types.PLimits, dm docs.Manager) (Manager, error) {
	b, err := backend.FromOptions(backendOptions)
	if err != nil {
		return nil, err
	}
	return &manager{
		b:       b,
		bucket:  bucket,
		dm:      dm,
		pLimits: pLimits,
	}, nil
}

type manager struct {
	b       backend.Backend
	bucket  string
	dm      docs.Manager
	pLimits types.PLimits
}

type pMapWorker func(
	ctx context.Context,
	projectId primitive.ObjectID,
	docId primitive.ObjectID,
) error

type pMapProducer func(
	ctx context.Context,
	projectId primitive.ObjectID,
	limit int32,
) (docIds <-chan primitive.ObjectID, errors <-chan error)

func (m *manager) pMap(ctx context.Context, projectId primitive.ObjectID, producer pMapProducer, worker pMapWorker) error {
	pCtx, done := context.WithCancel(ctx)
	defer done()
	docIds, errs := producer(pCtx, projectId, m.pLimits.BatchSize)

	l := sync.Mutex{}
	var firstError error

	setErr := func(err error) {
		l.Lock()
		defer l.Unlock()

		if firstError == nil {
			firstError = err
			done()
		}
	}
	doWork := func(docId primitive.ObjectID) error {
		if err := worker(pCtx, projectId, docId); err != nil {
			setErr(fmt.Errorf("failed for %v: %w", docId, err))
			return err
		}
		return nil
	}

	workerFinished := make(chan bool)
	workerSpawned := 0
	for workerSpawned < int(m.pLimits.ParallelArchiveJobs) {
		if firstError != nil {
			break
		}
		select {
		case docId, gotWork := <-docIds:
			if !gotWork {
				break
			}
			if err := doWork(docId); err != nil {
				break
			}
		case <-pCtx.Done():
			setErr(pCtx.Err())
			break
		case err, gotError := <-errs:
			if gotError {
				setErr(err)
			}
			break
		}

		// spawn new worker
		workerSpawned += 1
		go func() {
			for docId := range docIds {
				if err := doWork(docId); err != nil {
					break
				}
			}

			workerFinished <- true
		}()
	}
	for i := 0; i < workerSpawned; i++ {
		select {
		case <-workerFinished:
			continue
		case <-pCtx.Done():
			setErr(pCtx.Err())
			break
		case err, gotError := <-errs:
			if gotError {
				setErr(err)
			}
			break
		}
	}
	return firstError
}

func (m *manager) ArchiveDocs(ctx context.Context, projectId primitive.ObjectID) error {
	return m.pMap(ctx, projectId, m.dm.GetDocIdsForArchiving, m.ArchiveDoc)
}

func docKey(projectId primitive.ObjectID, docId primitive.ObjectID) string {
	return fmt.Sprintf("%s/%s", projectId.Hex(), docId.Hex())
}

type archivedDocBase struct {
	SchemaVersion int64 `json:"schema_v"`
}

type archivedDocV1 struct {
	archivedDocBase
	Lines  models.Lines  `json:"lines"`
	Ranges models.Ranges `json:"ranges"`
}

type archivedDocV0 models.Lines

func (m *manager) ArchiveDoc(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID) error {
	doc, err := m.dm.GetDocForArchiving(
		ctx,
		projectId,
		docId,
	)
	if err != nil {
		if errors.IsDocArchivedError(err) {
			// Race condition: Already archived.
			return nil
		}
		return err
	}

	if doc.Lines == nil {
		return fmt.Errorf("doc has no lines")
	}

	archivedDoc := archivedDocV1{}
	archivedDoc.SchemaVersion = 1
	archivedDoc.Lines = doc.Lines
	archivedDoc.Ranges = doc.Ranges

	key := docKey(projectId, docId)
	blob, err := json.Marshal(archivedDoc)
	reader := bytes.NewBuffer(blob)
	sendOptions := backend.SendOptions{
		ContentSize: int64(len(blob)),
	}
	err = m.b.SendFromStream(ctx, m.bucket, key, reader, sendOptions)
	if err != nil {
		return err
	}

	return m.dm.MarkDocAsArchived(ctx, projectId, docId, doc.Revision)
}

func (m *manager) UnArchiveDocs(ctx context.Context, projectId primitive.ObjectID) error {
	return m.pMap(ctx, projectId, m.dm.GetDocIdsForUnArchiving, m.UnArchiveDoc)
}

func (m *manager) UnArchiveDoc(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID) error {
	isArchived, err := m.dm.IsDocArchived(ctx, projectId, docId)
	if err != nil {
		return err
	}
	if !isArchived {
		// Race condition: already unarchived.
		return nil
	}
	key := docKey(projectId, docId)

	getOptions := backend.GetOptions{}
	reader, err := m.b.GetReadStream(ctx, m.bucket, key, getOptions)
	if err != nil {
		if !errors.IsNotFoundError(err) {
			return err
		}
		isArchivedNow, err2 := m.dm.IsDocArchived(ctx, projectId, docId)
		if err2 != nil {
			return err2
		}
		if !isArchivedNow {
			// race condition: another call has unarchived this doc.
			return nil
		}
		// cannot recover, fail with 404 from backend
		return err
	}
	blob, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	lines, ranges, err := deserializeArchive(blob)
	if err != nil {
		return err
	}
	_, err = m.dm.UpsertDoc(
		ctx,
		projectId,
		docId,
		lines,
		ranges,
	)
	if err != nil {
		return err
	}
	return m.b.DeleteObject(ctx, m.bucket, key)
}

func deserializeArchive(blob []byte) (models.Lines, models.Ranges, error) {
	var archiveV1 archivedDocV1
	if err := json.Unmarshal(blob, &archiveV1); err == nil {
		return archiveV1.Lines, archiveV1.Ranges, nil
	}
	var archiveV0 archivedDocV0
	if err := json.Unmarshal(blob, &archiveV0); err == nil {
		return models.Lines(archiveV0), models.Ranges{}, nil
	}
	return nil, models.Ranges{}, fmt.Errorf("unknown archive format")
}

func (m *manager) DestroyDocs(ctx context.Context, projectId primitive.ObjectID) error {
	return m.pMap(ctx, projectId, m.dm.GetDocIdsForDeletion, m.DestroyDoc)
}

func (m *manager) DestroyDoc(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID) error {
	err := m.b.DeleteObject(ctx, m.bucket, docKey(projectId, docId))
	if err != nil {
		if !errors.IsNotFoundError(err) {
			return err
		}
	}
	return m.dm.DestroyDoc(ctx, projectId, docId)
}
