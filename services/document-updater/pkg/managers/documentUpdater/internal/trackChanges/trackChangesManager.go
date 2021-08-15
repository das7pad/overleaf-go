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

package trackChanges

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/services/document-updater/pkg/errors"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater/internal/historyRedisManager"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
)

type Manager interface {
	FlushDocInBackground(projectId, docId primitive.ObjectID)
	RecordAndFlushHistoryOps(ctx context.Context, projectId, docId primitive.ObjectID, nUpdates, queueDepth int64) error
}

func New(options *types.Options, client redis.UniversalClient) (Manager, error) {
	baseURL, err := url.Parse(options.APIs.TrackChanges.URL)
	if err != nil {
		return nil, err
	}
	if baseURL.Scheme == "" {
		return nil, &errors.ValidationError{
			Msg: "trackChanges URL is missing scheme",
		}
	}
	return &manager{
		hrm:     historyRedisManager.New(client),
		baseURL: baseURL.String(),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

type manager struct {
	hrm historyRedisManager.Manager

	baseURL string
	client  *http.Client
}

const (
	flushEvery = 100
)

func shouldFlush(nUpdates, queueDepth int64) bool {
	if nUpdates < 1 || queueDepth < 1 {
		return false
	}
	before := (queueDepth - nUpdates) / flushEvery
	after := queueDepth / flushEvery
	return before != after
}

func (m *manager) FlushDocInBackground(projectId, docId primitive.ObjectID) {
	go m.flushDocChangesAndLogErr(projectId, docId)
}

func (m *manager) RecordAndFlushHistoryOps(ctx context.Context, projectId, docId primitive.ObjectID, nUpdates, queueDepth int64) error {
	if err := m.hrm.RecordDocHasHistory(ctx, projectId, docId); err != nil {
		return err
	}

	if shouldFlush(nUpdates, queueDepth) {
		m.FlushDocInBackground(projectId, docId)
	}
	return nil
}

func (m *manager) flushDocChangesAndLogErr(projectId, docId primitive.ObjectID) {
	err := m.flushDocChanges(context.Background(), projectId, docId)
	if err != nil {
		ids := projectId.Hex() + "/" + docId.Hex()
		err = errors.Tag(err, "cannot flush history for "+ids)
		log.Println(err.Error())
	}
}

func (m *manager) flushDocChanges(ctx context.Context, projectId, docId primitive.ObjectID) error {
	u := m.baseURL
	u += "/project/" + projectId.Hex()
	u += "/doc/" + docId.Hex()
	u += "/flush"
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	if err != nil {
		return err
	}
	res, err := m.client.Do(r)
	if err != nil {
		return err
	}
	_ = res.Body.Close()
	switch res.StatusCode {
	case http.StatusNoContent:
		return nil
	default:
		return errors.New(
			"non-success status code from track-changes: " + res.Status,
		)
	}
}
