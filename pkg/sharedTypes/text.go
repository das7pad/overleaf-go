// Golang port of Overleaf
// Copyright (C) 2021-2023 Jakob Ackermann <das7pad@outlook.com>
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

package sharedTypes

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"unicode"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type Hash string

func (h Hash) CheckMatches(other Hash) error {
	if h == other {
		return nil
	}
	return &errors.CodedError{
		Msg: string("snapshot hash mismatch: " + h + " != " + other),
	}
}

const (
	MaxDocLength = 2 * 1024 * 1024
	// MaxDocSizeBytes allows for 4 bytes per unicode character.
	MaxDocSizeBytes = 4 * MaxDocLength
)

var (
	ErrDocIsTooLarge       = &errors.ValidationError{Msg: "doc is too large"}
	ErrDocHasNullChar      = &errors.ValidationError{Msg: "doc has null char"}
	ErrDocHasSurrogateChar = &errors.ValidationError{Msg: "doc has surrogate char"}
)

type Snapshot []rune

func (s Snapshot) CheckSize() error {
	if len(s) > MaxDocLength {
		return ErrDocIsTooLarge
	}
	return nil
}

func (s Snapshot) Validate() error {
	if err := s.CheckSize(); err != nil {
		return err
	}
	for _, r := range s {
		if r == '\x00' {
			return ErrDocHasNullChar
		}
		if unicode.Is(unicode.Cs, r) {
			return ErrDocHasSurrogateChar
		}
	}
	return nil
}

func (s *Snapshot) UnmarshalJSON(bytes []byte) error {
	var raw string
	if err := json.Unmarshal(bytes, &raw); err != nil {
		return err
	}
	*s = Snapshot(raw)
	return nil
}

func (s Snapshot) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}

func (s Snapshot) Hash() Hash {
	d := sha1.New()
	d.Write([]byte("blob "))
	d.Write([]byte(strconv.FormatInt(int64(len(s)), 10)))
	d.Write([]byte{0})
	d.Write([]byte(string(s)))
	return Hash(hex.EncodeToString(d.Sum(nil)))
}

func (s Snapshot) Slice(start, end int) Snippet {
	l := len(s)
	if l < start {
		return Snippet("")
	}
	if l < end {
		end = l
	}
	return Snippet(s[start:end])
}

type Snippet []rune

func (s Snippet) Equals(other Snippet) bool {
	if len(s) != len(other) {
		return false
	}
	for i, r := range s {
		if other[i] != r {
			return false
		}
	}
	return true
}

func (s *Snippet) UnmarshalJSON(bytes []byte) error {
	var raw string
	if err := json.Unmarshal(bytes, &raw); err != nil {
		return err
	}
	*s = Snippet(raw)
	return nil
}

func (s Snippet) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}
