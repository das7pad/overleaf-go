// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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

	"github.com/redis/go-redis/v9"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

const (
	sessionIdSizeBytes = 24
	sessionIdKeyPrefix = "sess:"
)

var (
	b64                  = base64.RawURLEncoding
	sessionIdSizeEncoded = b64.EncodedLen(sessionIdSizeBytes)
)

type Id string

func (s Id) toKey() string {
	return sessionIdKeyPrefix + string(s)
}

func (s Id) toSessionValidationToken() sessionValidationToken {
	return sessionValidationToken("v1:" + s[len(s)-4:])
}

func genNewSessionId() (Id, error) {
	b := make([]byte, sessionIdSizeBytes)
	if _, err := rand.Read(b); err != nil {
		return "", errors.Tag(err, "generate new session id")
	}
	return Id(b64.EncodeToString(b)), nil
}

func (s *Session) newSessionId(ctx context.Context) (Id, []byte, error) {
	var err error
	var id Id
	var blob []byte
	var ok bool

	for i := 0; i < 10; i++ {
		id, err = genNewSessionId()
		if err != nil {
			continue
		}
		blob, err = s.serializeWithId(id)
		if err != nil {
			return "", nil, err
		}
		ok, err = s.client.SetNX(ctx, id.toKey(), blob, s.expiry).Result()
		if err != nil {
			err = errors.Tag(err, "set in redis")
			continue
		}
		if !ok {
			err = errors.New("id already taken")
			continue
		}
		if s.IsLoggedIn() {
			// Multi/EXEC skips over nil error from `SET NX`.
			// Perform tracking calls after getting session id.
			_, err = s.client.TxPipelined(ctx, func(tx redis.Pipeliner) error {
				trackingKey := userSessionsKey(s.User.Id)
				tx.SAdd(ctx, trackingKey, id.toKey())
				tx.Expire(ctx, trackingKey, s.expiry)
				return nil
			})
			if err != nil {
				return "", nil, errors.Tag(err, "track session")
			}
		}
		return id, blob, nil
	}
	return "", nil, err
}

func userSessionsKey(id sharedTypes.UUID) string {
	b := make([]byte, 0, 14+36+1)
	b = append(b, "UserSessions:{"...)
	b = id.Append(b)
	b = append(b, '}')
	return string(b)
}
