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

func tokenize(s, f string) tokens {
	if len(s) == 0 {
		return nil
	}
	var a tokens
	var start int32
	var k, l kind
	for i, c := range s {
		switch c {
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
		case '\\':
			k = tokenBackslash
		case '.':
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
		if i > 0 {
			a = append(a, token{
				kind:  l,
				start: start,
				f:     f,
				v:     s[start:i],
			})
			start = int32(i)
		}
		l = k
	}
	a = append(a, token{
		kind:  l,
		start: start,
		f:     f,
		v:     s[start:],
	})
	return a
}
