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

package compile

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func getPersistenceKey(options sharedTypes.ProjectOptions) string {
	//goland:noinspection SpellCheckingInspection
	return "clsiserver" +
		":" + string(options.CompileGroup) +
		":" + options.ProjectId.String() +
		":" + options.UserId.String()
}

func (m *manager) persistenceDisabled() bool {
	return m.persistenceCookieName == ""
}

func (m *manager) populateServerIdFromResponse(ctx context.Context, res *http.Response, options sharedTypes.ProjectOptions) types.ClsiServerId {
	if m.persistenceDisabled() {
		return ""
	}
	var clsiServerId types.ClsiServerId
	for _, cookie := range res.Cookies() {
		if cookie.Name == m.persistenceCookieName {
			clsiServerId = types.ClsiServerId(cookie.Value)
			break
		}
	}
	k := getPersistenceKey(options)
	persistenceTTL := m.persistenceTTL
	if clsiServerId == "" {
		// Bump expiry of persistence in the background.
		// It's ok to switch the backend occasionally.
		go func() {
			backgroundCtx, done := context.WithTimeout(
				context.Background(), time.Second*10,
			)
			err := m.client.Expire(backgroundCtx, k, persistenceTTL).Err()
			done()
			if err != nil {
				log.Printf("bump clsi persistence: %s", err.Error())
			}
		}()
	} else {
		// Race-Condition: Switch backend in foreground.
		// We want to go to the same backend on re-compile.
		err := m.client.Set(ctx, k, string(clsiServerId), persistenceTTL).Err()
		if err != nil {
			// Persistence is a performance optimization and ok to fail.
			log.Printf("update clsi persistence: %s", err.Error())
		}
	}
	return clsiServerId
}

func (m *manager) getServerId(ctx context.Context, options sharedTypes.ProjectOptions) (types.ClsiServerId, error) {
	if m.persistenceDisabled() {
		return "", nil
	}
	k := getPersistenceKey(options)
	s, err := m.client.Get(ctx, k).Result()
	if err != nil && err != redis.Nil {
		return "", errors.Tag(err, "get persistence id from redis")
	}
	return types.ClsiServerId(s), nil
}

func (m *manager) clearServerId(ctx context.Context, options sharedTypes.ProjectOptions) error {
	if m.persistenceDisabled() {
		return nil
	}
	k := getPersistenceKey(options)
	err := m.client.Del(ctx, k).Err()
	if err != nil && err != redis.Nil {
		return errors.Tag(err, "clear persistence in redis")
	}
	return nil
}

func (m *manager) doPersistentRequest(ctx context.Context, options sharedTypes.ProjectOptions, clsiServerId types.ClsiServerId, r *http.Request) (*http.Response, types.ClsiServerId, error) {
	if clsiServerId != "" {
		r.AddCookie(&http.Cookie{
			Name:  m.persistenceCookieName,
			Value: string(clsiServerId),
		})
	}
	res, err := m.pool.Do(r)
	if err != nil {
		return nil, clsiServerId, err
	}
	newClsiServerId := m.populateServerIdFromResponse(ctx, res, options)
	if newClsiServerId != "" {
		clsiServerId = newClsiServerId
	}
	return res, clsiServerId, nil
}

//goland:noinspection SpellCheckingInspection
const ClsiServerIdQueryParam = "clsiserverid"

func (m *manager) doStaticRequest(clsiServerId types.ClsiServerId, r *http.Request) (*http.Response, error) {
	q := r.URL.Query()
	q.Set(ClsiServerIdQueryParam, string(clsiServerId))
	r.URL.RawQuery = q.Encode()
	return m.pool.Do(r)
}
