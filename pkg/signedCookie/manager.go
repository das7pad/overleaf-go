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

package signedCookie

import (
	"crypto/sha256"
	"net/http"
	"net/url"
	"strings"

	"github.com/das7pad/overleaf-go/pkg/httpUtils"
)

type Manager interface {
	Get(c *httpUtils.Context) (string, error)
	Set(c *httpUtils.Context, s string)
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
	posDot        int
	size          int
	signingSecret []byte
	secrets       [][]byte
}

func (m *manager) Get(c *httpUtils.Context) (string, error) {
	v, err := c.Request.Cookie(m.Name)
	if err != nil {
		if err == http.ErrNoCookie {
			return "", ErrNoCookie
		}
		return "", err
	}
	s := v.Value
	if s == "" {
		return "", ErrNoCookie
	}
	if strings.HasPrefix(s, "s%3A") && len(s) <= 3*m.size {
		// Legacy cookie value.
		s, _ = url.QueryUnescape(s)
	}

	if len(s) != m.size || s[m.posDot] != '.' || s[0] != 's' || s[1] != ':' {
		return "", ErrBadCookieFormat
	}
	return m.unSign(s)
}

func (m *manager) Set(c *httpUtils.Context, s string) {
	var v string
	var expiry int
	if s == "" {
		v = ""
		expiry = -1
	} else {
		v = m.sign(s)
		expiry = int(m.Expiry.Seconds())
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     m.Name,
		Value:    v,
		Path:     m.Path,
		Domain:   m.Domain,
		MaxAge:   expiry,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
