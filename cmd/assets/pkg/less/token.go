// Golang port of Overleaf
// Copyright (C) 2023 Jakob Ackermann <das7pad@outlook.com>
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

package less

import (
	"fmt"
	"strings"
)

//go:generate stringer -type=kind

type kind int16

const (
	space kind = iota
	tokenNewline
	tokenAt
	tokenBackslash
	tokenBracketClose
	tokenBracketOpen
	tokenColon
	tokenCurlyClose
	tokenCurlyOpen
	tokenDot
	tokenExclamation
	tokenGt
	tokenHash
	tokenIdentifier
	tokenPercent
	tokenLt
	tokenMinus
	tokenUnderscore
	tokenParensClose
	tokenParensOpen
	tokenPlus
	tokenQuestion
	tokenComma
	tokenSemi
	tokenSlash
	tokenStar
	tokenEq
	tokenNum
	tokenSingleQuote
	tokenDoubleQuote
	tokenTilde
	tokenAmp

	compGt
	compGte
	compLt
	compLte
	compEq
	compNEq
)

type token struct {
	kind
	start int32
	f     string
	v     string
}

func (t token) String() string {
	return fmt.Sprintf("%s@%s@%d: %q", t.kind, t.f, t.start, t.v)
}

func (t token) IsSpace() bool {
	return t.kind == space || t.kind == tokenNewline
}

type tokens []token

func (tt tokens) String() string {
	if len(tt) == 0 {
		return ""
	}
	if len(tt) == 1 {
		return tt[0].v
	}
	n := 0
	for _, t := range tt {
		n += len(t.v)
	}
	b := strings.Builder{}
	b.Grow(n)
	for _, t := range tt {
		b.WriteString(t.v)
	}
	return b.String()
}

func (tt tokens) WriteString(b *strings.Builder) {
	for _, t := range tt {
		b.WriteString(t.v)
	}
}

func (tt tokens) Eq(other tokens) bool {
	if len(tt) != len(other) {
		return false
	}
	for i, t := range tt {
		if t != other[i] {
			return false
		}
	}
	return true
}

type matchers []tokens

func (mm matchers) Eq(other []tokens) bool {
	if len(mm) != len(other) {
		return false
	}
	for i, m := range mm {
		if !m.Eq(other[i]) {
			return false
		}
	}
	return true
}

func tokenize(s, f string) tokens {
	if len(s) == 0 {
		return nil
	}
	var a tokens
	var start int32
	var skip int
	var k, l kind
	for i := 0; i < len(s); i++ {
		if skip != 0 {
			skip = 0
		}
		switch s[i] {
		case ' ':
			if l == space {
				continue
			}
			k = space
		case '\n':
			if l == tokenNewline {
				continue
			}
			k = tokenNewline
		case '/':
			k = tokenSlash
			if (i == 0 || s[i-1] != '\\') && len(s) > i+1 {
				if s[i+1] == '/' {
					skip = strings.IndexRune(s[i+1:], '\n')
					if skip == -1 {
						skip = len(s) - i
					} else {
						skip += 2
					}
				} else if s[i+1] == '*' {
					skip = strings.Index(s[i+2:], "*/")
					if skip == -1 {
						skip = 0
					} else {
						skip += 4
					}
				}
			}
		case '\\':
			k = tokenBackslash
		case '.':
			if l == tokenNum {
				continue
			}
			k = tokenDot
		case '-':
			if l == tokenIdentifier {
				continue
			}
			if l == tokenAt {
				k = tokenIdentifier
			} else {
				k = tokenMinus
			}
		case '_':
			if l == tokenIdentifier {
				continue
			}
			if l == tokenAt {
				k = tokenIdentifier
			} else {
				k = tokenUnderscore
			}
		case ',':
			k = tokenComma
		case ':':
			k = tokenColon
		case ';':
			k = tokenSemi
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			if l == tokenIdentifier || l == tokenNum {
				continue
			}
			if l == tokenAt {
				k = tokenIdentifier
			} else {
				k = tokenNum
			}
		case '@':
			k = tokenAt
		case '(':
			k = tokenParensOpen
		case ')':
			k = tokenParensClose
		case '#':
			k = tokenHash
		case '[':
			k = tokenBracketOpen
		case ']':
			k = tokenBracketClose
		case '{':
			k = tokenCurlyOpen
		case '}':
			k = tokenCurlyClose
		case '>':
			k = tokenGt
		case '<':
			k = tokenLt
		case '=':
			k = tokenEq
		case '?':
			k = tokenQuestion
		case '!':
			k = tokenExclamation
		case '*':
			k = tokenStar
		case '+':
			k = tokenPlus
		case '~':
			k = tokenTilde
		case '%':
			k = tokenPercent
		case '&':
			k = tokenAmp
		case '\'':
			k = tokenSingleQuote
		case '"':
			k = tokenDoubleQuote
		default:
			if l == tokenIdentifier {
				continue
			}
			k = tokenIdentifier
		}
		if start != int32(i) {
			a = append(a, token{
				kind:  l,
				start: start,
				f:     f,
				v:     s[start:i],
			})
			start = int32(i)
		}
		if skip != 0 {
			i += skip - 1
			start += int32(skip)
		}
		l = k
	}
	if int(start) < len(s) {
		a = append(a, token{
			kind:  l,
			start: start,
			f:     f,
			v:     s[start:],
		})
	}
	return a
}
