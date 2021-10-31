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

package session

import (
	"bytes"
	"context"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type internalDataAccessOnly = Data

type Session struct {
	// Block full access on the session data. Read/Write individual details.
	*internalDataAccessOnly

	persistedId Id
	persisted   []byte
	client      redis.UniversalClient
	expiry      time.Duration
	id          Id
	noAutoSave  bool
}

func (s *Session) SetNoAutoSave() {
	s.noAutoSave = true
}

func (s *Session) Cycle(ctx context.Context) error {
	noAutoSafeBefore := s.noAutoSave
	s.noAutoSave = true
	oldId := s.id

	r, err := s.assignNewSessionId(ctx)
	if err != nil {
		return err
	}

	if s.User != nil && !s.User.Id.IsZero() {
		// Multi/EXEC skips over nil error from `SET NX`.
		// Perform tracking calls after getting session id.
		_, err2 := s.client.TxPipelined(ctx, func(tx redis.Pipeliner) error {
			key := userSessionsKey(s.User.Id)
			tx.SAdd(ctx, key, string(r.id))
			tx.Expire(ctx, key, s.expiry)
			return nil
		})
		if err2 != nil {
			return err2
		}
	}

	if oldId != "" {
		// Session cleanup can happen in the background; Ignore errors as well.
		go func() {
			delCtx, done :=
				context.WithTimeout(context.Background(), 10*time.Second)
			defer done()
			_ = destroySession(delCtx, s.client, oldId)
		}()
	}

	// Populate after tracking the session.
	r.Populate(s)
	s.noAutoSave = noAutoSafeBefore
	return nil
}

func destroySession(ctx context.Context, client redis.UniversalClient, id Id) error {
	if id == "" {
		return nil
	}
	err := client.Del(ctx, id.toKey()).Err()
	if err != nil && err != redis.Nil {
		return err
	}
	return nil
}

func (s *Session) Destroy(ctx context.Context) error {
	return destroySession(ctx, s.client, s.id)
}

func (s *Session) Save(ctx context.Context) (bool, error) {
	if s.id == "" {
		r, err := s.assignNewSessionId(ctx)
		if err != nil {
			return false, err
		}
		r.Populate(s)
		return false, nil
	}
	b, err := serializeSession(s.id, s.internalDataAccessOnly)
	if err != nil {
		return false, errors.Tag(err, "cannot serialize session")
	}

	if bytes.Equal(b, s.persisted) {
		// Same content, skip redis operation.
		return true, nil
	}

	if err = s.client.SetXX(ctx, s.id.toKey(), b, s.expiry).Err(); err != nil {
		return false, errors.Tag(err, "cannot update session data")
	}
	s.persisted = b
	return false, nil
}
