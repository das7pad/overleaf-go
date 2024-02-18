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

package signedCookie

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

var (
	ErrNoCookie           = &errors.UnauthorizedError{Reason: "no cookie found"}
	ErrBadCookieFormat    = &errors.UnauthorizedError{Reason: "bad cookie format"}
	ErrBadCookieSignature = &errors.UnauthorizedError{Reason: "bad cookie signature"}
)

func (m *manager) sign(s string) string {
	return "s:" + s + "." + string(m.genHMAC(s, m.signingSecret))
}

func (m *manager) unSign(raw string) (string, error) {
	v := raw[2:m.posDot]
	actualHMAC := []byte(raw[m.posDot+1:])
	for _, secret := range m.secrets {
		expectedHMAC := m.genHMAC(v, secret)
		if hmac.Equal(actualHMAC, expectedHMAC) {
			return v, nil
		}
	}
	return "", ErrBadCookieSignature
}

func (m *manager) genHMAC(s string, key []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(s))
	digest := h.Sum(nil)
	return base64.RawStdEncoding.AppendEncode(nil, digest)
}
