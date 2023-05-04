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

package dispatchManager

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"

	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater/internal/docManager"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
)

type Manager interface {
	ProcessDocumentUpdates(ctx context.Context)
	QueueUpdate(ctx context.Context, projectId, docId sharedTypes.UUID, update sharedTypes.DocumentUpdate) error
}

const (
	maxProcessingTime = 30 * time.Second
)

func New(options *types.Options, client redis.UniversalClient, dm docManager.Manager) Manager {
	return &manager{
		client:                       client,
		dm:                           dm,
		pendingUpdatesListShardCount: options.PendingUpdatesListShardCount,
		workersPerShard:              options.Workers,
	}
}

type manager struct {
	client                       redis.UniversalClient
	dm                           docManager.Manager
	pendingUpdatesListShardCount int
	workersPerShard              int
}

func (m *manager) GetPendingUpdatesListKey() types.PendingUpdatesListKey {
	shard := rand.Intn(m.pendingUpdatesListShardCount)
	return types.PendingUpdatesListKey(shard)
}

func (m *manager) ProcessDocumentUpdates(ctx context.Context) {
	queue := make(chan string, m.pendingUpdatesListShardCount)

	workerWg := sync.WaitGroup{}
	for i := 0; i < m.workersPerShard; i++ {
		workerWg.Add(1)
		go func() {
			m.worker(queue)
			workerWg.Done()
		}()
	}

	producerWg := sync.WaitGroup{}
	for i := 0; i < m.pendingUpdatesListShardCount; i++ {
		key := types.PendingUpdatesListKey(i).String()
		producerWg.Add(1)
		go func() {
			m.producer(ctx, queue, key)
			producerWg.Done()
		}()
	}
	producerWg.Wait()
	close(queue)
	workerWg.Wait()
}

func (m *manager) producer(ctx context.Context, queue chan<- string, key string) {
	for {
		res, err := m.client.BLPop(ctx, 0, key).Result()
		if err == context.Canceled {
			break
		}
		if err != nil {
			err = errors.Tag(err, "cannot get work from redis list "+key)
			log.Println(err.Error())
			continue
		}
		// res[0] is the key we popped from, aka `key`
		// res[1] is the value we popped
		queue <- res[1]
	}
}

func parseKey(key string) (sharedTypes.UUID, sharedTypes.UUID, error) {
	if len(key) != 36+36+1 {
		return sharedTypes.UUID{}, sharedTypes.UUID{}, &errors.ValidationError{
			Msg: "unexpected length",
		}
	}
	projectId, err := sharedTypes.ParseUUID(key[:36])
	if err != nil {
		return sharedTypes.UUID{}, sharedTypes.UUID{}, err
	}
	docId, err := sharedTypes.ParseUUID(key[36+1:])
	if err != nil {
		return sharedTypes.UUID{}, sharedTypes.UUID{}, err
	}
	return projectId, docId, nil
}

func (m *manager) worker(queue <-chan string) {
	for key := range queue {
		projectId, docId, err := parseKey(key)
		if err != nil {
			err = errors.Tag(err, fmt.Sprintf("unexpected key %q", key))
			log.Println(err.Error())
			continue
		}

		ctx, cancel := context.WithTimeout(
			context.Background(), maxProcessingTime,
		)
		err = m.dm.ProcessUpdatesForDocHeadless(ctx, projectId, docId)
		cancel()
		if err != nil {
			ids := projectId.String() + "/" + docId.String()
			err = errors.Tag(err, ids)
			log.Println(err.Error())
		}
	}
}

func (m *manager) QueueUpdate(ctx context.Context, projectId, docId sharedTypes.UUID, update sharedTypes.DocumentUpdate) error {
	// Hard code document id
	update.DocId = docId
	// Dup is an output only field
	update.Dup = false
	// Ingestion time is tracked internally only
	now := time.Now()
	update.Meta.IngestionTime = &now

	if err := update.Validate(); err != nil {
		return err
	}

	blob, err := json.Marshal(update)
	if err != nil {
		return errors.Tag(err, "encode update")
	}

	pendingUpdateKey := "PendingUpdates:{" + docId.String() + "}"
	if err = m.client.RPush(ctx, pendingUpdateKey, blob).Err(); err != nil {
		return errors.Tag(err, "queue update")
	}

	shardKey := m.GetPendingUpdatesListKey().String()
	docKey := projectId.String() + ":" + docId.String()
	if err = m.client.RPush(ctx, shardKey, docKey).Err(); err != nil {
		return errors.Tag(err, "notify shard about new queue entry")
	}
	return nil
}
