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
	"crypto/hmac"
	"crypto/sha256"
	"hash"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/signedCookie"
)

type Manager interface {
	GetSession(c *gin.Context) (*Session, error)
	GetOrCreateSession(c *gin.Context) (*Session, error)
	GetSessionById(ctx context.Context, id Id) (*Session, error)
	Flush(c *gin.Context, session *Session) error
	RequireLoggedInSession(c *gin.Context) (*Session, error)
}

func New(options signedCookie.Options, client redis.UniversalClient) Manager {
	verifier := make([]hash.Hash, 0, len(options.Secrets))
	for _, s := range options.Secrets {
		verifier = append(verifier, hmac.New(sha256.New, []byte(s)))
	}
	return &manager{
		signedCookie: signedCookie.New(options, sessionIdSizeEncoded),
		client:       client,
		Options:      options,
		signer:       verifier[0],
		verifier:     verifier,
	}
}

type manager struct {
	signedCookie.Options
	signedCookie signedCookie.Manager
	client       redis.UniversalClient
	signer       hash.Hash
	verifier     []hash.Hash
}

const (
	timingKeyGet = "session.timing.get"
)

var (
	timerStartGet = httpUtils.StartTimer(timingKeyGet)
	timerEndGet   = httpUtils.EndTimer(timingKeyGet, "session")
)
var cookieTriggerCyclingOfCSRFSecret = (&http.Cookie{
	Name:     "_csrf",
	Value:    "",
	Path:     "/",
	MaxAge:   -1,
	Secure:   true,
	HttpOnly: true,
	SameSite: http.SameSiteLaxMode,
}).String()

func (m *manager) new(id Id, persisted []byte, data *Data) *Session {
	if data.User == nil {
		data.User = anonymousUser
	}
	return &Session{
		client:                 m.client,
		expiry:                 m.Expiry,
		id:                     id,
		incomingUserId:         &data.User.Id,
		internalDataAccessOnly: data,
		persisted:              persisted,
		providedId:             id,
	}
}

func (m *manager) RequireLoggedInSession(c *gin.Context) (*Session, error) {
	sess, err := m.GetSession(c)
	if err != nil {
		if err == redis.Nil || err == signedCookie.ErrNoCookie {
			return nil, &errors.UnauthorizedError{
				Reason: "no session found",
			}
		}
		return nil, err
	}
	if err = sess.CheckIsLoggedIn(); err != nil {
		return nil, err
	}
	return sess, nil
}

func (m *manager) GetSession(c *gin.Context) (*Session, error) {
	timerStartGet(c)
	defer timerEndGet(c)

	id, err := m.signedCookie.Get(c)
	if err != nil {
		return nil, err
	}
	return m.GetSessionById(c.Request.Context(), Id(id))
}

func (m *manager) GetSessionById(c context.Context, id Id) (*Session, error) {
	raw, err := m.client.Get(c, id.toKey()).Bytes()
	if err != nil {
		return nil, err
	}
	data, err := deSerializeSession(id, raw)
	if err != nil {
		return nil, err
	}
	sess := m.new(id, raw, data)
	return sess, nil
}

func (m *manager) GetOrCreateSession(c *gin.Context) (*Session, error) {
	sess, err := m.GetSession(c)
	if err == redis.Nil || err == signedCookie.ErrNoCookie {
		sess = m.new("", nil, &Data{})
		return sess, nil
	}
	return sess, err
}

func (m *manager) Flush(c *gin.Context, session *Session) error {
	if !session.noAutoSave {
		skipped, err := session.Save(c.Request.Context())
		if err != nil {
			return err
		}
		if skipped || session.id == session.providedId {
			return nil
		}
	} else if session.id == session.providedId && session.id != "" {
		return nil
	}
	m.signedCookie.Set(c, string(session.id))

	// Trigger cycling of the csrf secret when cycling/clearing session id.
	c.Writer.Header().Add("Set-Cookie", cookieTriggerCyclingOfCSRFSecret)
	return nil
}
