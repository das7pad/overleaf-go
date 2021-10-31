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

package signedCookie

import (
	"crypto/sha256"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

type Manager interface {
	Get(c *gin.Context) (string, error)
	Set(c *gin.Context, s string)
}

func New(options Options, payloadLength int) Manager {
	secrets := make([][]byte, len(options.Secrets))
	for i, secret := range options.Secrets {
		secrets[i] = []byte(secret)
	}
	posDoc := len("s:") + payloadLength
	size := posDoc + len(".") + b64.EncodedLen(sha256.Size)
	return &manager{
		Options:       options,
		posDot:        posDoc,
		size:          size,
		signingSecret: secrets[0],
		secrets:       secrets,
	}
}

type manager struct {
	Options
	client        redis.UniversalClient
	posDot        int
	size          int
	signingSecret []byte
	secrets       [][]byte
}

func (m *manager) Get(c *gin.Context) (string, error) {
	s, err := c.Cookie(m.Name)
	if err != nil {
		if err == http.ErrNoCookie {
			return "", ErrNoCookie
		}
		return "", err
	}

	if len(s) != m.size || s[m.posDot] != '.' || s[0] != 's' || s[1] != ':' {
		return "", ErrBadCookieFormat
	}
	return m.unSign(s)
}

func (m *manager) Set(c *gin.Context, s string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		m.Name,
		m.sign(s),
		int(m.Expiry.Seconds()),
		m.Path,
		m.Domain,
		true,
		true,
	)
}
