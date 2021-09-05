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

package compile

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func getPersistenceKey(options types.SignedCompileProjectRequestOptions) string {
	return fmt.Sprintf(
		"clsiserver:%s:%s:%s",
		options.CompileGroup,
		options.ProjectId.Hex(),
		options.UserId.Hex(),
	)
}

func (m *manager) populateServerIdFromResponse(ctx context.Context, res *http.Response, options types.SignedCompileProjectRequestOptions) (types.ClsiServerId, error) {
	if m.options.APIs.Clsi.Persistence.CookieName == "" {
		return "", nil
	}
	var clsiServerId types.ClsiServerId
	for _, cookie := range res.Cookies() {
		if cookie.Name == m.options.APIs.Clsi.Persistence.CookieName {
			clsiServerId = types.ClsiServerId(cookie.Value)
			break
		}
	}
	k := getPersistenceKey(options)
	persistenceTTL := m.options.APIs.Clsi.Persistence.TTL
	var err error
	if clsiServerId == "" {
		err = m.client.Expire(ctx, k, persistenceTTL).Err()
	} else {
		err = m.client.Set(ctx, k, string(clsiServerId), persistenceTTL).Err()
	}
	return clsiServerId, err
}

func (m *manager) assignNewServerId(ctx context.Context, options types.SignedCompileProjectRequestOptions) (types.ClsiServerId, error) {
	u := m.baseURL
	u += "/project/" + options.ProjectId.Hex()
	u += "/user/" + options.UserId.Hex()
	u += "/status"
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	if err != nil {
		return "", errors.Tag(err, "cannot create cookie fetch request")
	}
	res, err := m.pool.Do(r)
	if err != nil {
		return "", errors.Tag(err, "cannot action cookie fetch request")
	}

	return m.populateServerIdFromResponse(ctx, res, options)
}

func (m *manager) getServerId(ctx context.Context, options types.SignedCompileProjectRequestOptions) (types.ClsiServerId, error) {
	if m.options.APIs.Clsi.Persistence.CookieName == "" {
		return "", nil
	}
	k := getPersistenceKey(options)
	s, err := m.client.Get(ctx, k).Result()
	if err != nil && err != redis.Nil {
		return "", errors.Tag(err, "cannot get persistence id from redis")
	}
	if s != "" {
		return types.ClsiServerId(s), nil
	}
	return m.assignNewServerId(ctx, options)
}

func (m *manager) clearServerId(ctx context.Context, options types.SignedCompileProjectRequestOptions) error {
	if m.options.APIs.Clsi.Persistence.CookieName == "" {
		return nil
	}
	k := getPersistenceKey(options)
	err := m.client.Del(ctx, k).Err()
	if err != nil && err != redis.Nil {
		return errors.Tag(err, "cannot clear persistence in redis")
	}
	return nil
}

func (m *manager) doPersistentRequest(ctx context.Context, options types.SignedCompileProjectRequestOptions, r *http.Request) (*http.Response, types.ClsiServerId, error) {
	clsiServerId, err := m.getServerId(ctx, options)
	if err != nil {
		return nil, "", err
	}
	if clsiServerId != "" {
		r.AddCookie(&http.Cookie{
			Name:  m.options.APIs.Clsi.Persistence.CookieName,
			Value: string(clsiServerId),
		})
	}
	res, err := m.pool.Do(r)
	if err != nil {
		return nil, clsiServerId, err
	}
	newClsiServerId, err := m.populateServerIdFromResponse(
		ctx, res, options,
	)
	if err != nil {
		// Backend persistence is a performance optimization.
		// It is ok to fail. We received a response, why discard it now?
		log.Printf("cannot update clsi persistence: %s", err.Error())
	}
	if newClsiServerId != "" {
		clsiServerId = newClsiServerId
	}
	return res, clsiServerId, nil
}

//goland:noinspection SpellCheckingInspection
const clsiServerIdQueryParam = "clsiserverid"

func (m *manager) doStaticRequest(clsiServerId types.ClsiServerId, r *http.Request) (*http.Response, error) {
	r.URL.Query().Set(clsiServerIdQueryParam, string(clsiServerId))
	return m.pool.Do(r)
}
