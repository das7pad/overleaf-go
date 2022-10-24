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

package session

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type internalDataAccessOnly = Data

type Session struct {
	// Block full access on the session data. Read/Write individual details.
	*internalDataAccessOnly

	client     redis.UniversalClient
	expiry     time.Duration
	id         Id
	noAutoSave bool
	persisted  []byte
	providedId Id
}

func (s *Session) CheckIsLoggedIn() error {
	if !s.IsLoggedIn() {
		return ErrNotLoggedIn
	}
	return nil
}

func (s *Session) IsLoggedIn() bool {
	return s.User.Id != (sharedTypes.UUID{})
}

func (s *Session) Login(ctx context.Context, u user.ForSession, ip string) (string, error) {
	redirect, triggerCleanup, err := s.PrepareLogin(ctx, u, ip)
	if err != nil {
		return "", err
	}
	triggerCleanup()
	return redirect, nil
}

func (s *Session) PrepareLogin(ctx context.Context, u user.ForSession, ip string) (string, func(), error) {
	redirect := s.PostLoginRedirect
	triggerCleanup := s.prepareCleanup()
	s.noAutoSave = true
	s.PostLoginRedirect = ""
	s.User = &User{
		Id:             u.Id,
		FirstName:      u.FirstName,
		LastName:       u.LastName,
		Email:          u.Email,
		Epoch:          u.Epoch,
		IPAddress:      ip,
		SessionCreated: time.Now(),
	}
	s.AnonTokenAccess = nil
	id, blob, err := s.newSessionId(ctx)
	if err != nil {
		return "", nil, errors.Tag(err, "cannot cycle session")
	}
	if redirect == "" {
		redirect = "/project"
	}
	return redirect, func() {
		s.id = id
		s.persisted = blob
		if triggerCleanup == nil {
			return
		}
		// Session cleanup can happen in the background; Ignore errors as well.
		go func() {
			delCtx, done := context.WithTimeout(
				context.Background(), 10*time.Second,
			)
			defer done()
			_ = triggerCleanup(delCtx)
		}()
	}, nil
}

func (s *Session) prepareCleanup() func(ctx context.Context) error {
	if s.id == "" {
		return nil
	}
	id := s.id
	userId := s.User.Id

	return func(ctx context.Context) error {
		err := s.client.Del(ctx, id.toKey()).Err()
		if err != nil && err != redis.Nil {
			return err
		}
		if userId == (sharedTypes.UUID{}) {
			return nil
		}
		// Multi/EXEC skips over nil error from `DEL`.
		// NOTE: The session may reside in another redis cluster shard than
		//        the user tracking key. Sending both commands in parallel has
		//        the potential for scrubbing the tracking key but leaving the
		//        session alive.
		// Perform tracking calls after deleting session id.
		// Ignore errors as there is no option to recover from any error
		//  (e.g. retry logging out) as the actual session data has been
		//  cleared already.
		go func() {
			bCtx, done := context.WithTimeout(
				context.Background(), 3*time.Second,
			)
			defer done()
			s.client.SRem(bCtx, userSessionsKey(userId), id.toKey())
		}()
		return nil
	}
}

func (s *Session) Destroy(ctx context.Context) error {
	cleanup := s.prepareCleanup()
	// Any following access/writes must error out.
	s.internalDataAccessOnly = nil
	s.noAutoSave = true
	s.persisted = nil
	if cleanup != nil {
		if err := cleanup(ctx); err != nil {
			return errors.Tag(err, "destroy session")
		}
	}
	s.id = ""
	return nil
}

type OtherSessionData struct {
	IPAddress      string    `json:"ip_address"`
	SessionCreated time.Time `json:"session_created"`
}

type OtherSessionsDetails struct {
	Sessions   []OtherSessionData
	sessionIds []Id
}

func (s *Session) GetOthers(ctx context.Context) (*OtherSessionsDetails, error) {
	if err := s.CheckIsLoggedIn(); err != nil {
		return nil, err
	}

	allKeys, err := s.client.SMembers(ctx, userSessionsKey(s.User.Id)).Result()
	if err != nil {
		return nil, errors.Tag(err, "cannot get list of sessions")
	}

	otherIds := make([]Id, 0, len(allKeys))
	otherSessionKeys := make([]string, 0, len(allKeys))
	for _, raw := range allKeys {
		id := Id(strings.TrimPrefix(raw, "sess:"))
		if id == s.id {
			continue
		}
		otherIds = append(otherIds, id)
		otherSessionKeys = append(otherSessionKeys, id.toKey())
	}

	if len(otherIds) == 0 {
		return &OtherSessionsDetails{}, nil
	}

	sessionData := make([]*redis.StringCmd, len(otherSessionKeys))
	_, err = s.client.Pipelined(ctx, func(p redis.Pipeliner) error {
		for i, key := range otherSessionKeys {
			sessionData[i] = p.Get(ctx, key)
		}
		return nil
	})
	if err != nil && err != redis.Nil {
		return nil, errors.Tag(err, "cannot fetch session data")
	}
	sessions := make([]OtherSessionData, 0, len(otherIds))
	validSessionIds := make([]Id, 0, len(otherIds))
	for i, id := range otherIds {
		var blob []byte
		if blob, err = sessionData[i].Bytes(); err != nil {
			if err == redis.Nil {
				// Already deleted
				continue
			}
			return nil, errors.Tag(err, "cannot fetch session data")
		}
		data := sessionDataWithDeSyncProtection{}
		if err = json.Unmarshal(blob, &data); err != nil {
			return nil, errors.New("redis returned corrupt session data")
		}
		if err = data.ValidationToken.Validate(id); err != nil {
			return nil, err
		}
		if data.User == nil || data.User.Id != s.User.Id {
			// race-condition with logout and login of another user.
			continue
		}
		validSessionIds = append(validSessionIds, id)
		sessions = append(sessions, OtherSessionData{
			IPAddress:      data.User.IPAddress,
			SessionCreated: data.User.SessionCreated,
		})
	}
	return &OtherSessionsDetails{
		Sessions:   sessions,
		sessionIds: validSessionIds,
	}, nil
}

func (s *Session) DestroyOthers(ctx context.Context, d *OtherSessionsDetails) error {
	if len(d.sessionIds) == 0 {
		return errors.New("missing session ids")
	}
	if err := s.CheckIsLoggedIn(); err != nil {
		return err
	}

	_, err := s.client.Pipelined(ctx, func(p redis.Pipeliner) error {
		trackingKey := userSessionsKey(s.User.Id)
		for _, id := range d.sessionIds {
			key := id.toKey()
			p.Del(ctx, key)
			p.SRem(ctx, trackingKey, key)
		}
		return nil
	})
	if err != nil {
		return errors.Tag(err, "cannot clear session data")
	}
	return nil
}

func (s *Session) Touch(ctx context.Context) error {
	_, err := s.client.Pipelined(ctx, func(p redis.Pipeliner) error {
		p.Expire(ctx, s.id.toKey(), s.expiry)
		if s.IsLoggedIn() {
			p.Expire(ctx, userSessionsKey(s.User.Id), s.expiry)
		}
		return nil
	})
	return err
}

func (s *Session) Save(ctx context.Context) (bool, error) {
	if s.id == "" {
		if s.IsEmpty() {
			return true, nil
		}
		id, blob, err := s.newSessionId(ctx)
		if err != nil {
			return false, err
		}
		s.id = id
		s.persisted = blob
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
		return nil, errors.Tag(err, "serialize session")
	}
	return b, nil
}

func (s *Session) ToOtherSessionData() OtherSessionData {
	return OtherSessionData{
		IPAddress:      s.User.IPAddress,
		SessionCreated: s.User.SessionCreated,
	}
}
