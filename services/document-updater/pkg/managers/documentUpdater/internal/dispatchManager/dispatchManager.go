// Golang port of Overleaf
// Copyright (C) 2021-2022 Jakob Ackermann <das7pad@outlook.com>
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
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"

	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater/internal/docManager"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
)

type Manager interface {
	Start(ctx context.Context)
}

func New(options *types.Options, client redis.UniversalClient, dm docManager.Manager) Manager {
	return &manager{
		client: client,
		dm:     dm,
		o:      options,
	}
}

type manager struct {
	client redis.UniversalClient
	dm     docManager.Manager
	o      *types.Options
}

func (m *manager) Start(ctx context.Context) {
	for i := 0; i < m.o.PendingUpdatesListShardCount; i++ {
		queue := types.PendingUpdatesListKey(i).String()
		s := &shard{
			queue:   queue,
			workers: m.o.WorkersPerShard,
			client:  m.client,
			dm:      m.dm,
		}
		go s.run(ctx)
	}
}

type shard struct {
	queue   string
	workers int
	client  redis.UniversalClient
	dm      docManager.Manager
}

const (
	maxProcessingTime = 30 * time.Second
)

func (m *shard) run(ctx context.Context) {
	work := make(chan string)
	defer close(work)

	for i := 0; i < m.workers; i++ {
		go m.process(work)
	}
	for {
		res, err := m.client.BLPop(ctx, 0, m.queue).Result()
		if err == context.Canceled {
			break
		}
		if err != nil {
			err = errors.Tag(err, "cannot get work from queue "+m.queue)
			log.Println(err.Error())
			continue
		}
		key := res[1]
		work <- key
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

func (m *shard) process(work chan string) {
	for key := range work {
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
