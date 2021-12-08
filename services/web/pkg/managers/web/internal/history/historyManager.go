// Golang port of Overleaf
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

package history

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	GetProjectHistoryUpdates(ctx context.Context, request *types.GetProjectHistoryUpdatesRequest, response *types.GetProjectHistoryUpdatesResponse) error
	GetDocDiff(ctx context.Context, request *types.GetDocDiffRequest, response *types.GetDocDiffResponse) error
	RestoreDocVersion(ctx context.Context, request *types.RestoreDocVersionRequest) error
}

func New(options *types.Options, um user.Manager) Manager {
	return &manager{
		base: options.APIs.TrackChanges.URL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		um: um,
	}
}

type manager struct {
	base   sharedTypes.URL
	client *http.Client
	um     user.Manager
}

func (m *manager) apiCall(ctx context.Context, userId primitive.ObjectID, method string, url sharedTypes.URL, dst interface{}) error {
	r, err := http.NewRequestWithContext(ctx, method, url.String(), nil)
	if err != nil {
		return errors.Tag(err, "compose request")
	}
	r.Header.Add("X-User-Id", userId.Hex())
	res, err := m.client.Do(r)
	if err != nil {
		return errors.Tag(err, "perform request")
	}
	defer func() {
		_ = res.Body.Close()
	}()
	switch res.StatusCode {
	case http.StatusOK:
		if err = json.NewDecoder(res.Body).Decode(dst); err != nil {
			return &errors.UnprocessableEntityError{Msg: err.Error()}
		}
		return nil
	case http.StatusNoContent:
		return nil
	default:
		return &errors.UnprocessableEntityError{Msg: "upstream error"}
	}
}

func (m *manager) GetProjectHistoryUpdates(ctx context.Context, r *types.GetProjectHistoryUpdatesRequest, res *types.GetProjectHistoryUpdatesResponse) error {
	u := m.base.WithPath(fmt.Sprintf(
		"/project/%s/updates", r.ProjectId.Hex(),
	))
	query := url.Values{
		"min_count": {r.MinCount.String()},
	}
	if r.Before != 0 {
		query.Set("before", strconv.FormatInt(int64(r.Before), 10))
	}
	u = u.WithQuery(query)
	if err := m.apiCall(ctx, r.UserId, http.MethodGet, u, res); err != nil {
		return err
	}
	userIds := make(user.UniqUserIds, len(res.Updates))
	for _, entry := range res.Updates {
		for _, id := range entry.Meta.UserIds {
			userIds[id] = true
		}
	}
	users, err := m.um.GetUsersForBackFillingNonStandardId(ctx, userIds)
	if err != nil {
		return errors.Tag(err, "cannot get users")
	}
	for _, entry := range res.Updates {
		entry.Meta.Users = make(
			[]*user.WithPublicInfoAndNonStandardId, 0, len(entry.Meta.UserIds),
		)
		for _, id := range entry.Meta.UserIds {
			if usr := users[id]; usr != nil {
				entry.Meta.Users = append(entry.Meta.Users, usr)
			}
		}
		entry.Meta.UserIds = nil
	}
	return nil
}

func (m *manager) GetDocDiff(ctx context.Context, r *types.GetDocDiffRequest, res *types.GetDocDiffResponse) error {
	u := m.base.WithPath(fmt.Sprintf(
		"/project/%s/doc/%s/diff",
		r.ProjectId.Hex(), r.DocId.Hex(),
	))
	u = u.WithQuery(url.Values{
		"from": {r.From.String()},
		"to":   {r.To.String()},
	})
	if err := m.apiCall(ctx, r.UserId, http.MethodGet, u, res); err != nil {
		return err
	}
	userIds := make(user.UniqUserIds, len(res.Diff))
	for _, entry := range res.Diff {
		if entry.Meta != nil && entry.Meta.UserId != nil {
			userIds[*entry.Meta.UserId] = true
		}
	}
	users, err := m.um.GetUsersForBackFillingNonStandardId(ctx, userIds)
	if err != nil {
		return errors.Tag(err, "cannot get users")
	}
	for _, entry := range res.Diff {
		if entry.Meta == nil {
			continue
		}
		if entry.Meta.UserId != nil {
			entry.Meta.User = users[*entry.Meta.UserId]
			entry.Meta.UserId = nil
		}
	}
	return nil
}

func (m *manager) RestoreDocVersion(ctx context.Context, r *types.RestoreDocVersionRequest) error {
	u := m.base.WithPath(fmt.Sprintf(
		"/project/%s/doc/%s/version/%d/restore",
		r.ProjectId.Hex(), r.DocId.Hex(), r.FromV,
	))
	return m.apiCall(ctx, r.UserId, http.MethodPost, u, nil)
}
