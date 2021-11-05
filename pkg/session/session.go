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
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type internalDataAccessOnly = Data

type Session struct {
	// Block full access on the session data. Read/Write individual details.
	*internalDataAccessOnly

	client         redis.UniversalClient
	expiry         time.Duration
	id             Id
	incomingUserId *primitive.ObjectID
	noAutoSave     bool
	persisted      []byte
	providedId     Id
}

var errNotLoggedIn = &errors.UnauthorizedError{}

func (s *Session) CheckIsLoggedIn() error {
	if !s.IsLoggedIn() {
		return errNotLoggedIn
	}
	return nil
}

func (s *Session) IsLoggedIn() bool {
	return !s.User.Id.IsZero()
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

	if s.IsLoggedIn() {
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
			_ = s.destroyOldSession(delCtx, oldId)
		}()
	}

	// Populate after tracking the session.
	r.Populate(s)
	s.noAutoSave = noAutoSafeBefore
	return nil
}

func (s *Session) destroyOldSession(ctx context.Context, id Id) error {
	if id == "" {
		return nil
	}
	err := s.client.Del(ctx, id.toKey()).Err()
	if err != nil && err != redis.Nil {
		return err
	}
	if !s.incomingUserId.IsZero() {
		// Multi/EXEC skips over nil error from `DEL`.
		// Perform tracking calls after deleting session id.
		_, err2 := s.client.TxPipelined(ctx, func(tx redis.Pipeliner) error {
			key := userSessionsKey(*s.incomingUserId)
			tx.SRem(ctx, key, string(id))
			tx.Expire(ctx, key, s.expiry)
			return nil
		})
		if err2 != nil {
			return err2
		}
	}
	return nil
}

func (s *Session) Destroy(ctx context.Context) error {
	// Any following access/writes must error out.
	s.internalDataAccessOnly = nil
	s.noAutoSave = true
	s.persisted = nil
	if err := s.destroyOldSession(ctx, s.id); err != nil {
		return err
	}
	s.id = ""
	return nil
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
	b, err := s.serializeWithId(s.id)
	if err != nil {
		return false, err
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

func (s *Session) serializeWithId(id Id) ([]byte, error) {
	data := *s.internalDataAccessOnly
	if data.User == anonymousUser {
		data.User = nil
	}
	b, err := serializeSession(id, data)
	if err != nil {
		return b, errors.Tag(err, "cannot serialize session")
	}
	return b, nil
}
