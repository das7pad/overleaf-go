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
	"sync"
)

//go:generate stringer -type=kind

type kind int8

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

	comment

	compGt
	compGte
	compLt
	compLte
	compEq
	compNEq
)

type token struct {
	kind
	f      int16
	line   int16
	column int16
	v      string
}

func (t token) String() string {
	return fmt.Sprintf("%s@%d:%d:%d: %q", t.kind, t.f, t.line, t.column, t.v)
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

type cachedTokens struct {
	s  string
	tt tokens
}

func newTokenizer() *tokenizer {
	return &tokenizer{
		files: make(map[string]int16),
		m:     make(map[int16]cachedTokens),
	}
}

type tokenizer struct {
	mu    sync.RWMutex
	files map[string]int16
	m     map[int16]cachedTokens
}

func (r *tokenizer) ResolveFile(t token) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for s, fId := range r.files {
		if t.f == fId {
			return s
		}
	}
	return ""
}

func (r *tokenizer) Tokenize(s, f string) (tokens, int16) {
	r.mu.RLock()
	fId, ok := r.files[f]
	if !ok {
		r.mu.RUnlock()
		r.mu.Lock()
		fId, ok = r.files[f]
		if !ok {
			fId = int16(len(r.files)) + 1
			r.files[f] = fId
		}
		r.mu.Unlock()
		r.mu.RLock()
	}
	cached, ok := r.m[fId]
	r.mu.RUnlock()
	if ok && cached.s == s {
		return cached.tt, fId
	}
	cached.s = s
	cached.tt = tokenize(s, fId)
	r.mu.Lock()
	r.m[fId] = cached
	r.mu.Unlock()
	return cached.tt, fId
}

func tokenize(s string, f int16) tokens {
	if len(s) == 0 {
		return nil
	}
	length := int32(len(s))
	var tt tokens
	var start int32
	var skip int32
	var k, lastK kind
	line := int16(0)
	column := int16(0)
	for i := int32(0); i < length; i++ {
		switch s[i] {
		case ' ':
			if lastK == space {
				continue
			}
			k = space
		case '\n':
			if lastK == tokenNewline {
				continue
			}
			k = tokenNewline
		case '/':
			k = tokenSlash
			if (lastK != tokenBackslash) && length > i+1 {
				if s[i+1] == '/' {
					skip = int32(strings.IndexRune(s[i+1:], '\n'))
					if skip == -1 {
						skip = length - i
					} else {
						skip += 2
					}
					k = comment
				} else if s[i+1] == '*' {
					skip = int32(strings.Index(s[i+2:], "*/"))
					if skip == -1 {
						skip = 0
					} else {
						skip += 4
					}
					k = comment
				}
			}
		case '\\':
			k = tokenBackslash
		case '.':
			if lastK == tokenNum {
				continue
			}
			k = tokenDot
		case '-':
			if lastK == tokenIdentifier {
				continue
			}
			if lastK == tokenAt {
				k = tokenIdentifier
			} else {
				k = tokenMinus
			}
		case '_':
			if lastK == tokenIdentifier {
				continue
			}
			if lastK == tokenAt {
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
			if lastK == tokenIdentifier || lastK == tokenNum {
				continue
			}
			if lastK == tokenAt {
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
			if lastK == tokenIdentifier {
				continue
			}
			k = tokenIdentifier
		}
		if start != i {
			tt = append(tt, token{
				kind:   lastK,
				f:      f,
				column: column,
				line:   line,
				v:      s[start:i],
			})
			if lastK == tokenNewline {
				line += int16(i - start)
				column = 0
			} else {
				column += int16(i - start)
			}
			start = i
		}
		if k == comment {
			idx := int32(strings.LastIndexByte(s[start:i+skip], '\n'))
			if idx == -1 {
				column += int16(i - start + skip)
			} else {
				line += int16(strings.Count(s[start:i+skip], "\n"))
				column = int16(i - start + skip - idx - 1)
			}
			i += skip - 1
			start += skip
			skip = 0
		}
		lastK = k
	}
	if start < length {
		tt = append(tt, token{
			kind:   lastK,
			f:      f,
			line:   line,
			column: column,
			v:      s[start:],
		})
	}
	return tt
}
