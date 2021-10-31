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
	"context"
	"crypto/rand"
	"encoding/base64"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const sessionIdSizeBytes = 24

var b64 = base64.RawURLEncoding
var sessionIdSizeEncoded = b64.EncodedLen(sessionIdSizeBytes)

type Id string

func (s Id) toKey() string {
	return "sess:" + string(s)
}

func (s Id) toSessionValidationToken() sessionValidationToken {
	return sessionValidationToken("v1:" + s[len(s)-4:])
}

func genNewSessionId() (Id, error) {
	b := make([]byte, sessionIdSizeBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return Id(b64.EncodeToString(b)), nil
}

func (s *Session) assignNewSessionId(ctx context.Context) (*newSessionIdResult, error) {
	var err error
	var r *newSessionIdResult

	for i := 0; i < 10; i++ {
		r, err = s.newSessionIdOnceVia(ctx, s.client)
		if err != nil {
			continue
		}
		if err = r.Err(); err != nil {
			continue
		}
		return r, nil
	}
	return nil, err
}

type newSessionIdResult struct {
	cmdSet *redis.BoolCmd
	id     Id
	blob   []byte
}

func (r *newSessionIdResult) Err() error {
	return r.cmdSet.Err()
}

func (r *newSessionIdResult) Populate(s *Session) {
	s.id = r.id
	s.persisted = r.blob
}

func userSessionsKey(id primitive.ObjectID) string {
	return "UserSessions:{" + id.Hex() + "}"
}

func (s *Session) newSessionIdOnceVia(ctx context.Context, runner redis.Cmdable) (*newSessionIdResult, error) {
	id, err := genNewSessionId()
	if err != nil {
		return nil, err
	}
	blob, err := serializeSession(id, s.internalDataAccessOnly)
	if err != nil {
		return nil, err
	}

	cmdSet := runner.SetNX(ctx, id.toKey(), blob, s.expiry)

	return &newSessionIdResult{
		cmdSet: cmdSet,
		id:     id,
		blob:   blob,
	}, nil
}
