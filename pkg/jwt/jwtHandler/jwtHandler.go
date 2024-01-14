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
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/jwt/expiringJWT"
	"github.com/das7pad/overleaf-go/pkg/options/jwtOptions"
)

type JWT = expiringJWT.ExpiringJWT

type JWTHandler[T JWT] interface {
	New() T
	Parse(blob []byte) (T, error)
	SetExpiryAndSign(claims T) (string, error)
}

type NewClaims[T JWT] func() T

func New[T JWT](options jwtOptions.JWTOptions, newClaims NewClaims[T]) JWTHandler[T] {
	var method crypto.Hash
	switch options.Algorithm {
	case "HS256":
		method = crypto.SHA256
	case "HS384":
		method = crypto.SHA384
	case "HS512":
		method = crypto.SHA512
	}
	headerBlob := []byte(base64.RawURLEncoding.EncodeToString([]byte(
		`{"alg":"` + options.Algorithm + `","typ":"JWT"}`,
	)))
	return &handler[T]{
		expiresIn:  options.ExpiresIn,
		newClaims:  newClaims,
		key:        []byte(options.Key),
		method:     method,
		headerBlob: headerBlob,
	}
}

type handler[T JWT] struct {
	expiresIn  time.Duration
	key        []byte
	method     crypto.Hash
	newClaims  NewClaims[T]
	headerBlob []byte
}

func (h *handler[T]) New() T {
	return h.newClaims()
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

func (h *handler[T]) Parse(blob []byte) (T, error) {
	header, blob, hasHeader := bytes.Cut(blob, dotSeparator)
	payload, mac, hasPayload := bytes.Cut(blob, dotSeparator)
	if !hasHeader ||
		!hasPayload ||
		len(header) == 0 ||
		len(payload) == 0 ||
		len(mac) == 0 ||
		!bytes.Equal(header, h.headerBlob) {
		var tt T
		return tt, ErrTokenMalformed
	}

	m := hmac.New(h.method.New, h.key)
	{
		n, err := base64.RawURLEncoding.Decode(mac, mac)
		if err != nil {
			var tt T
			return tt, ErrTokenMalformed
		}
		mac = mac[:n]
	}
	m.Write(header[0 : len(header)+1+len(payload)])
	if !hmac.Equal(mac, m.Sum(header[:0])) {
		var tt T
		return tt, ErrSignatureInvalid
	}

	{
		n, err := base64.RawURLEncoding.Decode(payload, payload)
		if err != nil {
			var tt T
			return tt, ErrTokenMalformed
		}
		payload = payload[:n]
	}
	t := h.newClaims()
	if err := json.Unmarshal(payload, t); err != nil {
		var tt T
		return tt, ErrTokenMalformed
	}

	if err := t.Validate(); err != nil {
		var tt T
		return tt, err
	}
	return t, nil
}

func (h *handler[T]) SetExpiryAndSign(claims T) (string, error) {
	claims.SetExpiry(h.expiresIn)

	buf := b64Buffer{buf: make([]byte, 0, 384)}

	buf.AppendEncoded(h.headerBlob)
	buf.AppendEncoded(dotSeparator)

	e := json.NewEncoder(&buf)
	e.SetEscapeHTML(false)
	if err := e.Encode(claims); err != nil {
		return "", err
	}

	m := hmac.New(h.method.New, h.key)
	m.Write(buf.Bytes())

	buf.AppendEncoded(dotSeparator)
	if _, err := buf.Write(m.Sum(nil)); err != nil {
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
	n := len(b.buf)
	b.buf = b.buf[0 : n+base64.RawURLEncoding.EncodedLen(len(p))]
	base64.RawURLEncoding.Encode(b.buf[n:], p)
	return len(p), nil
}
