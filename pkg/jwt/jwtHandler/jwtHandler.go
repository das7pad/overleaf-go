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

package jwtHandler

import (
	"bytes"
	"crypto"
	"crypto/hmac"
	"encoding/base64"
	"encoding/json"
	"hash"
	"sync"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/jwt/expiringJWT"
	"github.com/das7pad/overleaf-go/pkg/options/jwtOptions"
)

type JWT = expiringJWT.ExpiringJWT

type NewClaims[T JWT] func() T

func New[T JWT](options jwtOptions.JWTOptions, newClaims NewClaims[T]) *JWTHandler[T] {
	var newHash func() hash.Hash
	switch options.Algorithm {
	case "HS256":
		newHash = crypto.SHA256.New
	case "HS384":
		newHash = crypto.SHA384.New
	case "HS512":
		newHash = crypto.SHA512.New
	}
	headerBlob := []byte(base64.RawURLEncoding.EncodeToString([]byte(
		`{"alg":"` + options.Algorithm + `","typ":"JWT"}`,
	)))
	key := []byte(options.Key)
	hmacSize := newHash().Size()
	hmacEncLen := base64.RawURLEncoding.EncodedLen(hmacSize)
	return &JWTHandler[T]{
		expiresIn:  options.ExpiresIn,
		newClaims:  newClaims,
		headerBlob: headerBlob,
		newHmac: func() hash.Hash {
			return hmac.New(newHash, key)
		},
		hmacOff:    uint32(hmacEncLen - hmacSize),
		hmacEncLen: uint32(hmacEncLen),
	}
}

type JWTHandler[T JWT] struct {
	expiresIn  time.Duration
	newClaims  NewClaims[T]
	headerBlob []byte
	hmacPool   sync.Pool
	newHmac    func() hash.Hash
	hmacOff    uint32
	hmacEncLen uint32
}

func (h *JWTHandler[T]) New() T {
	return h.newClaims()
}

type hmacPoolEntry struct {
	hmac hash.Hash
	buf  []byte
}

func (h *JWTHandler[T]) getHmac() *hmacPoolEntry {
	if v := h.hmacPool.Get(); v != nil {
		m := v.(*hmacPoolEntry)
		m.hmac.Reset()
		return m
	}
	m := h.newHmac()
	return &hmacPoolEntry{
		hmac: m,
		buf:  make([]byte, h.hmacEncLen),
	}
}

var (
	dotSeparator = []byte(".")

	ErrSignatureInvalid = &errors.UnauthorizedError{
		Reason: "jwt signature is invalid",
	}
	ErrTokenMalformed = &errors.UnauthorizedError{
		Reason: "jwt is malformed",
	}
)

func (h *JWTHandler[T]) Parse(blob []byte, now time.Time) (T, error) {
	payload, err := h.parsePayload(blob)
	if err != nil {
		var tt T
		return tt, err
	}
	t := h.newClaims()
	if err = t.FastUnmarshalJSON(payload); err != nil {
		var tt T
		return tt, ErrTokenMalformed
	}
	if err = t.Validate(now); err != nil {
		var tt T
		return tt, err
	}
	return t, nil
}

func (h *JWTHandler[T]) ParseInto(t T, blob []byte, now time.Time) error {
	payload, err := h.parsePayload(blob)
	if err != nil {
		return err
	}
	if err = t.FastUnmarshalJSON(payload); err != nil {
		return ErrTokenMalformed
	}
	if err = t.Validate(now); err != nil {
		return err
	}
	return nil
}

func (h *JWTHandler[T]) parsePayload(blob []byte) ([]byte, error) {
	header, blob, hasHeader := bytes.Cut(blob, dotSeparator)
	payload, mac, hasPayload := bytes.Cut(blob, dotSeparator)
	if !hasHeader ||
		!hasPayload ||
		len(header) == 0 ||
		len(payload) == 0 ||
		len(mac) != int(h.hmacEncLen) ||
		!bytes.Equal(header, h.headerBlob) {
		return nil, ErrTokenMalformed
	}

	m := h.getHmac()
	m.hmac.Write(header[0 : len(header)+1+len(payload)])
	m.hmac.Sum(m.buf[h.hmacOff:h.hmacOff])
	base64.RawURLEncoding.Encode(m.buf, m.buf[h.hmacOff:])
	ok := hmac.Equal(mac, m.buf)
	h.hmacPool.Put(m)
	if !ok {
		return nil, ErrSignatureInvalid
	}

	{
		n, err := base64.RawURLEncoding.Decode(payload, payload)
		if err != nil {
			return nil, ErrTokenMalformed
		}
		payload = payload[:n]
	}
	return payload, nil
}

func (h *JWTHandler[T]) SetExpiryAndSign(claims T) (string, error) {
	claims.SetExpiry(h.expiresIn)

	buf := b64Buffer{buf: make([]byte, 0, 384)}

	buf.AppendEncoded(h.headerBlob)
	buf.AppendEncoded(dotSeparator)

	e := json.NewEncoder(&buf)
	e.SetEscapeHTML(false)
	if err := e.Encode(claims); err != nil {
		return "", err
	}

	m := h.getHmac()
	m.hmac.Write(buf.Bytes())
	s := m.hmac.Sum(m.buf[:0])

	buf.AppendEncoded(dotSeparator)
	_, err := buf.Write(s)
	h.hmacPool.Put(m)
	if err != nil {
		return "", err
	}

	return string(buf.Bytes()), nil
}

type b64Buffer struct {
	buf []byte
}

func (b *b64Buffer) AppendEncoded(p []byte) {
	b.buf = append(b.buf, p...)
}

func (b *b64Buffer) Bytes() []byte {
	return b.buf
}

var newLineSuffix = []byte("\n")

func (b *b64Buffer) Write(p []byte) (int, error) {
	p = bytes.TrimSuffix(p, newLineSuffix)
	b.buf = base64.RawURLEncoding.AppendEncode(b.buf, p)
	return len(p), nil
}
