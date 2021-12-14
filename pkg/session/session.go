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
	"encoding/json"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
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
var errNotAdmin = &errors.NotAuthorizedError{}

func (s *Session) CheckIsAdmin() error {
	if err := s.CheckIsLoggedIn(); err != nil {
		return err
	}
	if !s.User.IsAdmin {
		return errNotAdmin
	}
	return nil
}

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

func (s *Session) Login(ctx context.Context, u *user.ForSession, ip string) (string, error) {
	redirect := s.PostLoginRedirect
	s.SetNoAutoSave()
	s.PostLoginRedirect = ""
	s.User = &User{
		Id:             u.Id,
		IsAdmin:        u.IsAdmin,
		FirstName:      u.FirstName,
		LastName:       u.LastName,
		Email:          u.Email,
		Epoch:          u.Epoch,
		ReferralId:     u.ReferralId,
		IPAddress:      ip,
		SessionCreated: time.Now().UTC(),
	}
	if err := s.Cycle(ctx); err != nil {
		return "", errors.Tag(err, "cannot cycle session")
	}
	if redirect == "" {
		redirect = "/project"
	}
	return redirect, nil
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
			tx.SAdd(ctx, key, r.id.toKey())
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
	key := id.toKey()
	err := s.client.Del(ctx, key).Err()
	if err != nil && err != redis.Nil {
		return err
	}
	if !s.incomingUserId.IsZero() {
		// Multi/EXEC skips over nil error from `DEL`.
		// Perform tracking calls after deleting session id.
		// Ignore errors as there is no option to recover from any error
		//  (e.g. retry logging out) as the actual session data has been
		//  cleared already.
		_ = s.client.SRem(ctx, userSessionsKey(*s.incomingUserId), key).Err()
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

type OtherSessionData struct {
	IPAddress      string    `json:"ip_address"`
	SessionCreated time.Time `json:"session_created"`
}

type OtherSessionsDetails struct {
	Sessions   []*OtherSessionData
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
	sessions := make([]*OtherSessionData, 0, len(otherIds))
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
		data := &sessionDataWithDeSyncProtection{}
		if err = json.Unmarshal(blob, data); err != nil {
			return nil, errors.New("redis returned corrupt session data")
		}
		if err = data.Validate(id); err != nil {
			return nil, err
		}
		if data.User == nil || data.User.Id != s.User.Id {
			// race-condition with logout and login of another user.
			continue
		}
		validSessionIds = append(validSessionIds, id)
		sessions = append(sessions, &OtherSessionData{
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
