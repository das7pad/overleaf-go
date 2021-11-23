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

package project

import (
	"crypto/rand"
	"crypto/subtle"
	"math/big"
	"strings"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

const (
	lenReadOnly           = 12
	lenReadAndWrite       = 22
	lenReadAndWritePrefix = 10
)

type AccessToken string

func (t AccessToken) EqualsTimingSafe(other AccessToken) bool {
	return subtle.ConstantTimeCompare([]byte(t), []byte(other)) == 1
}

var ErrInvalidTokenFormat = &errors.ValidationError{Msg: "invalid token format"}

func (t AccessToken) ValidateReadAndWrite() error {
	if len(t) != lenReadAndWrite {
		return ErrInvalidTokenFormat
	}
	for i := 0; i < lenReadAndWritePrefix; i++ {
		if t[i] < '0' || t[i] > '9' {
			return ErrInvalidTokenFormat
		}
	}
	for i := lenReadAndWritePrefix; i < lenReadAndWrite; i++ {
		if t[i] < 'a' || t[i] > 'z' {
			return ErrInvalidTokenFormat
		}
	}
	return nil
}

func (t AccessToken) ValidateReadOnly() error {
	if len(t) != lenReadOnly {
		return ErrInvalidTokenFormat
	}
	for i := 0; i < lenReadOnly; i++ {
		if t[i] < 'a' || t[i] > 'z' {
			return ErrInvalidTokenFormat
		}
	}
	return nil
}

type Tokens struct {
	ReadOnly           AccessToken `json:"readOnly" bson:"readOnly"`
	ReadAndWrite       AccessToken `json:"readAndWrite" bson:"readAndWrite"`
	ReadAndWritePrefix string      `json:"readAndWritePrefix" bson:"readAndWritePrefix"`
}

const (
	charsetAlpha    = "bcdfghjkmnpqrstvwxyz"
	charsetNumerics = "123456789"
)

var (
	sizeCharsetAlpha    = big.NewInt(int64(len(charsetAlpha)))
	sizeCharsetNumerics = big.NewInt(int64(len(charsetNumerics)))
)

func randomStringFrom(source string, max *big.Int, n int) (string, error) {
	// NOTE: This naive implementation w/o bulk random reads is fast enough.
	//       For all token types, p95 n=100 is in O(100µs) or 10k/s.
	var target strings.Builder
	for i := 0; i < n; i++ {
		v, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", errors.Tag(err, "cannot get random int")
		}
		target.WriteByte(source[v.Int64()])
	}
	return target.String(), nil
}

func generateTokens() (*Tokens, error) {
	ro, err := randomStringFrom(charsetAlpha, sizeCharsetAlpha, lenReadOnly)
	if err != nil {
		return nil, err
	}

	rwP, err := randomStringFrom(
		charsetNumerics, sizeCharsetNumerics, lenReadAndWritePrefix,
	)
	if err != nil {
		return nil, err
	}

	rwS, err := randomStringFrom(
		charsetAlpha, sizeCharsetAlpha, lenReadAndWrite-lenReadAndWritePrefix,
	)
	if err != nil {
		return nil, err
	}

	return &Tokens{
		ReadOnly:           AccessToken(ro),
		ReadAndWrite:       AccessToken(rwP + rwS),
		ReadAndWritePrefix: rwP,
	}, nil
}
