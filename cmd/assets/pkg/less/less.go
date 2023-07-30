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
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
)

func Parse(f string) (string, []string, error) {
	return ParseUsing(os.ReadFile, f)
}

func ParseUsing(read func(name string) ([]byte, error), f string) (string, []string, error) {
	p := parser{
		read: read,
	}
	if err := p.parse(f); err != nil {
		fmt.Println(p.printRaw())
		fmt.Println(p.print())
		return "", p.getImports(), err
	}
	fmt.Println(p.printRaw())
	s, err := p.print()
	if err != nil {
		fmt.Println(p.print())
		return "", p.getImports(), err
	}
	return s, p.getImports(), err
}

type directive struct {
	name  string
	value tokens
}

type node struct {
	matcher    tokens
	when       tokens
	directives []directive
	children   []node
	imports    []string
	vars       []map[string]tokens
	paramVars  map[string]tokens
	mixins     []map[string][]node
}

func consumeUntil(s tokens, needle ...kind) (int, error) {
	end := len(s) - len(needle) + 1
nextStart:
	for i := 0; i < end; i++ {
		for j := 0; j < len(needle); j++ {
			if s[i+j].kind != needle[j] {
				continue nextStart
			}
		}
		return i, nil
	}
	return -1, fmt.Errorf("%s not found", needle)
}

func maybeExpectSeq(tt tokens, j int, ignoreSpace bool, needle ...kind) (int, int) {
	for off, c := range needle {
		if ignoreSpace {
			j += consumeSpace(tt[j:])
		}
		if len(tt) < j+1 {
			return j, -1
		}
		if got := tt[j].kind; got != c {
			return j, off
		}
		j++
	}
	return j, 0
}

func expectSeq(tt tokens, j int, ignoreSpace bool, needle ...kind) (int, error) {
	j, r := maybeExpectSeq(tt, j, ignoreSpace, needle...)
	switch r {
	case 0:
		return j, nil
	case -1:
		return j, fmt.Errorf("expected sequence %s, ran out of tokens after %d", needle, j)
	default:
		c := needle[r]
		got := tt[j].kind
		return j, fmt.Errorf("expected sequence %s, wanted %q as token %d, got %q", needle, c, r, got)
	}
}

func consumeSpace(s tokens) int {
	for i, t := range s {
		switch t.kind {
		case space:
			continue
		case tokenNewline:
			continue
		}
		return i
	}
	return len(s)
}

func trimSpace(s tokens) tokens {
	s = s[consumeSpace(s):]
	for i := len(s) - 1; i >= 0; i-- {
		switch s[i].kind {
		case space:
			continue
		case tokenNewline:
			continue
		}
		return s[:i+1]
	}
	return s
}

func mergeSpace(s tokens) tokens {
	s = trimSpace(s)
	double := 0
	for i := 1; i < len(s); i++ {
		switch s[i].kind {
		case space, tokenNewline:
			switch s[i-1].kind {
			case space, tokenNewline:
				double++
			}
		}
	}
	if double == 0 {
		return s
	}
	out := make(tokens, 0, len(s)-double)
	out = append(out, s[0])

	for i := 1; i < len(s); i++ {
		switch s[i].kind {
		case space, tokenNewline:
			switch s[i-1].kind {
			case space, tokenNewline:
				continue
			}
			out = append(out, token{kind: space, v: " "})
			continue
		}
		out = append(out, s[i])
	}
	return out
}

func index(s tokens, k kind) int {
	for i, t := range s {
		if t.kind == k {
			return i
		}
	}
	return -1
}

func cut(s tokens, k kind) (tokens, tokens, bool) {
	if i := index(s, k); i != -1 {
		return s[:i], s[i+1:], true
	}
	return s, nil, false
}

func cutToken(s tokens, needle token) (tokens, tokens, bool) {
	for i, t := range s {
		if t.kind == needle.kind && t.v == needle.v {
			return s[:i], s[i+1:], true
		}
	}
	return s, nil, false
}

func parseComp(s tokens, i int) (int, kind, error) {
	i += consumeSpace(s[i:])
	if len(s) < i+1 {
		return 0, 0, errors.New("missing comparator")
	}
	var c kind
	switch s[i].kind {
	case tokenEq:
		c = compEq
	case tokenExclamation:
		c = tokenExclamation
	case tokenLt:
		c = compLt
	case tokenGt:
		c = compGt
	default:
		return i, 0, fmt.Errorf("wanted comp, got %s", s[i])
	}
	i++
	if len(s) == i || s[i].kind == space {
		if c == tokenExclamation {
			return i, 0, errors.New("solo ! comp")
		}
		return i, c, nil
	}

	switch s[i].kind {
	case tokenEq:
		switch c {
		case compLt:
			c = compLte
		case compGt:
			c = compGte
		case tokenExclamation:
			c = compNEq
		default:
			return i, 0, fmt.Errorf("comp %s followed by = comp", c)
		}
	case tokenLt:
		if c == compEq {
			c = compLte
		} else {
			return i, 0, fmt.Errorf("comp %s followed by < comp", c)
		}
	case tokenGt:
		if c == compEq {
			c = compGte
		} else {
			return i, 0, fmt.Errorf("comp %s followed by < comp", c)
		}
	default:
		if c == tokenExclamation {
			return i, 0, fmt.Errorf("token %s after ! comp", s[i])
		}
		return i, c, nil
	}
	return i + 1, c, nil
}

func compare[T float64 | string](a T, c kind, b T) bool {
	switch c {
	case compEq:
		if a != b {
			return false
		}
	case compNEq:
		if a == b {
			return false
		}
	case compLt:
		if a >= b {
			return false
		}
	case compGt:
		if a <= b {
			return false
		}
	case compLte:
		if a > b {
			return false
		}
	case compGte:
		if a < b {
			return false
		}
	}
	return true
}

func (n *node) branchNode() node {
	return node{
		vars:   append([]map[string]tokens{{}}, n.vars...),
		mixins: append([]map[string][]node{{}}, n.mixins...),
	}
}

var whenToken = token{kind: tokenIdentifier, v: "when"}

func (n *node) consume(f string, read func(name string) ([]byte, error), tt tokens, i int) (int, error) {
doneParsing:
	for ; i < len(tt); i++ {
	topLevelToken:
		switch tt[i].kind {
		case space, tokenNewline:
			continue
		case tokenCurlyClose:
			return i + 1, nil
		case tokenSlash:
			return i, errors.New("unexpected '/'")
		case tokenAt:
			if len(tt) > i+2 {
				j := i + 2
				j += consumeSpace(tt[j:])
				t1 := tt[i+1]
				switch t1.kind {
				case tokenCurlyOpen:
					if len(tt) > j+2 &&
						tt[j].kind == tokenIdentifier &&
						tt[j+1].kind == tokenCurlyClose {
						break topLevelToken
					}
				case tokenIdentifier:
					switch t1.v {
					case "import":
						if len(tt) > j+3 &&
							tt[j].kind == tokenParensOpen &&
							tt[j+1].kind == tokenIdentifier &&
							tt[j+1].v == "less" &&
							tt[j+2].kind == tokenParensClose {
							j += 3
						}
						j += consumeSpace(tt[j:])
						if len(tt) > j+1 &&
							tt[j].kind == tokenSingleQuote {
							j += 1
							k, err := consumeUntil(tt[j:], tokenSingleQuote, tokenSemi)
							if err != nil {
								return i, err
							}
							next := path.Join(
								path.Dir(f), tt[j:j+k].String(),
							)
							if path.Ext(next) == "" {
								next += ".less"
							}
							if err = n.parse(read, next); err != nil {
								return i, err
							}
							i = j + k + 2
							continue
						}
					case "charset":
						if len(tt) > j+3 &&
							tt[j].kind == tokenDoubleQuote &&
							tt[j+1].kind == tokenIdentifier &&
							tt[j+2].kind == tokenDoubleQuote {
							n.directives = append([]directive{
								{
									name:  "@charset",
									value: tt[j : j+3],
								},
							}, n.directives...)
							j += 3
							j += consumeSpace(tt[j:])
							if len(tt) < j+1 || tt[j].kind != tokenSemi {
								return i, errors.New("expected semi after charset")
							}
							i = j + 1
							continue
						}
					case
						"font-face",
						"keyframes", "-moz-keyframes", "-webkit-keyframes",
						"media",
						"viewport", "-ms-viewport":
						break topLevelToken
					default:
						if len(tt) > j+1 &&
							tt[j].kind == tokenColon {
							j += 1
							j += consumeSpace(tt[j:])
							k, err := consumeUntil(tt[j:], tokenSemi)
							if err != nil {
								return i, err
							}
							v := trimSpace(tt[j : j+k])
							if len(v) > 0 && v[0].kind == tokenCurlyOpen {
								// @foo: { color: red; }
								break topLevelToken
							}
							n.vars[0][t1.v] = v
							i = j + k
							continue
						}
						if len(tt) > j+1 &&
							tt[j].kind == tokenParensOpen {
							// @foo();
							break topLevelToken
						}
					}
				}
			}
			return i, errors.New("unexpected '@'")
		case tokenAmp:
			j, r := maybeExpectSeq(tt, i, true, tokenAmp, tokenColon, tokenIdentifier, tokenParensOpen, tokenDot, tokenIdentifier)
			if r == 0 && tt[i+2].v == "extend" {
				n.directives = append(n.directives, directive{
					value: tokens{tt[j-2], tt[j-1]},
				})
				j += consumeSpace(tt[j:])
				if len(tt) > j+1 &&
					tt[j].kind == tokenIdentifier &&
					tt[j].v == "all" {
					j++
				}
				var err error
				j, err = expectSeq(tt, j, true, tokenParensClose, tokenSemi)
				if err != nil {
					return j, err
				}
				i = j
				continue
			}
		case tokenIdentifier:
			if tt[i].v == "each" &&
				len(tt) > i &&
				tt[i+1].kind == tokenParensOpen {
				j := i + 2
				j += consumeSpace(tt[j:])
				j, err := expectSeq(tt, j, false, tokenAt, tokenIdentifier)
				if err != nil {
					return j, err
				}
				src := mergeSpace(tt[j-2 : j])
				j, err = expectSeq(tt, j, true, tokenComma, tokenCurlyOpen)
				if err != nil {
					return j, err
				}

				n1 := n.branchNode()
				n2 := n1.branchNode()
				j, err = n2.consume(f, read, tt, j)
				if err != nil {
					return j, err
				}
				j, err = expectSeq(tt, j, true, tokenParensClose, tokenSemi)
				if err != nil {
					return j, err
				}

				n2.matcher = tokens{
					{kind: tokenParensOpen, v: "("},
					{kind: tokenAt, v: "@"},
					{kind: tokenIdentifier, v: "key"},
					{kind: tokenComma, v: ","},
					{kind: tokenAt, v: "@"},
					{kind: tokenIdentifier, v: "value"},
					{kind: tokenParensClose, v: ")"},
				}
				n1.mixins[0][".each"] = []node{n2}
				n1.directives = append(n1.directives, directive{
					name:  "each",
					value: src,
				})
				n.children = append(n.children, n1)
				i = j
				continue
			}
		}
		// directive, mixin, matcher

		var j int
		var t2 token
		colon := -1
		when := -1
	nextToken:
		for j, t2 = range tt[i:] {
			switch t2.kind {
			case tokenParensOpen:
				if when != -1 || colon != -1 {
					continue
				}
				break nextToken
			case tokenCurlyOpen:
				if j > 0 && len(tt) > i+j+2 &&
					tt[i+j-1].kind == tokenAt &&
					tt[i+j+0].kind == tokenCurlyOpen &&
					tt[i+j+1].kind == tokenIdentifier &&
					tt[i+j+2].kind == tokenCurlyClose {
					continue
				}
				break nextToken
			case tokenSemi:
				break nextToken
			case tokenColon:
				colon = j
			case tokenNum:
			case tokenIdentifier:
				if t2.v == "when" {
					when = j
				}
			}
		}
		if t2.kind == tokenSemi {
			if colon == -1 {
				n.directives = append(n.directives, directive{
					value: mergeSpace(tt[i : i+j]),
				})
			} else {
				n.directives = append(n.directives, directive{
					name:  tt[i : i+colon].String(),
					value: mergeSpace(tt[i+colon+1 : i+j]),
				})
			}
			i += j
			continue
		}
		if t2.kind == tokenParensOpen {
			isAtMedia := tt[i].kind == tokenAt && tt[i+1].v == "media"
			l := 0
			end := -1
			for k, t3 := range tt[i+j:] {
				switch t3.kind {
				case tokenParensOpen:
					if end != -1 {
						return i + j, errors.New("unexpected ( after mixin args )")
					}
					l++
					continue
				case tokenParensClose:
					l--
					if l == 0 {
						end = i + j + k
					}
					continue
				case tokenSemi:
					if l == 0 {
						n.directives = append(n.directives, directive{
							value: mergeSpace(tt[i : end+1]),
						})
						i += j + k
						continue doneParsing
					}
					continue
				case tokenCurlyOpen:
					if k > 0 && len(tt) > i+j+k+2 &&
						tt[i+j+k-1].kind == tokenAt &&
						tt[i+j+k+0].kind == tokenCurlyOpen &&
						tt[i+j+k+1].kind == tokenIdentifier &&
						tt[i+j+k+2].kind == tokenCurlyClose {
						continue
					}
					j += k
					// mixin definition start
					break
				case space, tokenNewline:
					continue
				default:
					if l == 0 {
						if isAtMedia {
							switch t3.kind {
							case tokenComma:
								end = -1
								continue
							case tokenIdentifier:
								switch t3.v {
								case "and":
									end = -1
									continue
								case "only", "screen", "print":
									continue
								}
							}
						}
						if t3.kind == tokenIdentifier && t3.v == "when" {
							m, err := consumeUntil(tt[i+j+k:], tokenCurlyOpen)
							if err != nil {
								return i + j + k, err
							}
							j += k + m
							break
						}
						return i + j + k, errors.New("unexpected token after mixin args )")
					}
					continue
				}
				break
			}
		}
		isVarMixin := tt[i].kind == tokenAt &&
			tt[i+1].kind == tokenIdentifier &&
			index(tt[i+1+consumeSpace(tt[i+1:]):], tokenColon) == 1
		n1 := n.branchNode()
		n1.matcher, n1.when, _ = cutToken(tt[i:i+j], whenToken)
		n1.matcher = mergeSpace(n1.matcher)
		n1.when = trimSpace(n1.when)
		if isVarMixin {
			n1.matcher = append(
				n1.matcher[:len(n1.matcher)-1],
				token{kind: tokenParensOpen, v: "("},
				token{kind: tokenParensClose, v: ")"},
			)
			t2 = n1.matcher[len(n1.matcher)-2]
		}
		if t2.kind == tokenCurlyOpen &&
			index(n1.matcher, tokenColon) <
				index(n1.matcher, tokenParensOpen) {
			colon = index(n1.matcher, tokenColon)
			c := colon
			if len(n1.matcher) > c+3 &&
				n1.matcher[c+1].kind == tokenIdentifier &&
				n1.matcher[c+1].v == "extend" &&
				n1.matcher[c+2].kind == tokenParensOpen {
				c += 3
				c += consumeSpace(n1.matcher[c:])
				if len(n1.matcher) > c+2 &&
					n1.matcher[c].kind == tokenDot &&
					n1.matcher[c+1].kind == tokenIdentifier {
					n1.directives = append(n1.directives, directive{
						value: tokens{n1.matcher[c], n1.matcher[c+1]},
					})
					c += 2
					c += consumeSpace(n1.matcher[c:])
					if len(n1.matcher) > c+1 &&
						n1.matcher[c].kind == tokenIdentifier &&
						n1.matcher[c].v == "all" {
						c += 1
						c += consumeSpace(n1.matcher[c:])
					}
					if len(n1.matcher) != c+1 ||
						n1.matcher[c].kind != tokenParensClose {
						return i, errors.New("unexpected extend, missing )")
					}
					n1.matcher = trimSpace(n1.matcher[:colon])
				}
			}
		}
		var err error
		j, err = n1.consume(f, read, tt, i+j+1)
		switch t2.kind {
		case tokenParensOpen:
			argsStart := index(n1.matcher, tokenParensOpen)
			if argsStart == -1 {
				return i, errors.New("expected args")
			}
			name := trimSpace(n1.matcher[:argsStart]).String()
			n1.matcher = n1.matcher[argsStart:]
			n.mixins[0][name] = append(n.mixins[0][name], n1)
			if len(n.matcher) > 0 && n.matcher[0].kind == tokenHash {
				chain := n.matcher.String() + " > " + name
				n.mixins[len(n.mixins)-1][chain] = append(
					n.mixins[len(n.mixins)-1][chain], n1,
				)
			}
		case tokenCurlyOpen:
			n.children = append(n.children, n1)
		}
		if err != nil {
			return i, err
		}
		if isVarMixin {
			j, err = expectSeq(tt, j, true, tokenSemi)
			if err != nil {
				return j, nil
			}
		}
		i = j
	}
	return len(tt), nil
}

func isConstant(s tokens) bool {
	if len(s) == 0 {
		return false
	}
	for i, t := range s {
		switch t.kind {
		case tokenAt:
			return false
		case tokenPlus, tokenMinus, tokenStar, tokenSlash:
			return false
		case tokenParensOpen:
			if i == 0 || s[i-1].kind != tokenIdentifier {
				return false
			}
			switch s[i-1].v {
			case "calc":
			case "rgb", "rgba", "hsl", "hsla", "hwb":
			default:
				return false
			}
		}
	}
	return true
}

func isKeyframes(s tokens) bool {
	if len(s) == 0 {
		return false
	}
	if len(s) < 3 || s[0].kind != tokenAt || s[1].kind != tokenIdentifier {
		return false
	}
	switch s[1].v {
	case "keyframes", "-moz-keyframes", "-webkit-keyframes":
		return true
	}
	return false
}

func shouldNest(prev, s tokens) bool {
	if len(s) == 0 {
		return false
	}
	if len(s) == 1 {
		if isKeyframes(prev) &&
			s[0].kind == tokenIdentifier &&
			(s[0].v == "from" || s[0].v == "to") {
			return true
		}
		return false
	}
	if s[0].kind == tokenAt &&
		s[1].kind == tokenIdentifier {
		return true
	}
	if isKeyframes(prev) &&
		s[0].kind == tokenNum &&
		s[1].kind == tokenPercent {
		return true
	}
	return false
}

var stubParentMatcher = matchers{nil}

func (n *node) evalMatcher(pv []map[string]tokens, p tokens, mm matchers) (tokens, matchers) {
	matcher := n.evalVars(n.matcher, pv)
	if len(matcher) == 0 {
		return nil, mm
	}
	if shouldNest(p, matcher) {
		return matcher, mm
	}
	if len(mm) == 0 {
		return nil, buildMatchers(stubParentMatcher, matcher)
	}
	return nil, buildMatchers(mm, matcher)
}

func buildMatchers(mm matchers, matcher tokens) matchers {
	var out matchers
	var acc tokens
	for _, m := range mm {
		hasAmp := false
		for i, t := range matcher {
			switch t.kind {
			case tokenAmp:
				hasAmp = true
				if len(m) > 0 && m[0].kind != space {
					acc = append(acc, token{kind: space, v: " "})
				}
				acc = append(acc, m...)
			case tokenComma:
			default:
				acc = append(acc, t)
			}
			if len(acc) > 0 && (t.kind == tokenComma || len(matcher) == i+1) {
				if !hasAmp && len(m) > 0 {
					buf := make(tokens, 0, len(m)+1+len(acc))
					buf = append(buf, m...)
					if acc[0].kind != space {
						buf = append(buf, token{kind: space, v: " "})
					}
					buf = append(buf, acc...)
					acc = buf
				}
				out = append(out, trimSpace(acc))
				acc = nil
				hasAmp = false
			}
		}
	}
	return out
}

func (n *node) evalWhen(pv []map[string]tokens) (bool, error) {
	if len(n.when) == 0 {
		return true, nil
	}
	s := n.evalVars(n.when, pv)
	for len(s) > 0 {
		s = s[consumeSpace(s):]
		if len(s) == 0 || s[0].kind != tokenParensOpen {
			return false, errors.New("incomplete when, wanted (")
		}
		s = s[1:]
		s = s[consumeSpace(s):]

		if len(s) == 0 {
			return false, errors.New("incomplete when, wanted a=num/identifier")
		}
		if s[0].kind == tokenIdentifier {
			a := s[0]
			s = s[1:]
			s = s[consumeSpace(s):]

			i, c, err := parseComp(s, 0)
			if err != nil {
				return false, fmt.Errorf("unexpected when comp: %s", err)
			}
			s = s[i:]
			s = s[consumeSpace(s):]

			if len(s) == 0 || s[0].kind != tokenIdentifier {
				return false, errors.New("incomplete when, wanted identifier")
			}
			b := s[0]
			s = s[1:]
			s = s[consumeSpace(s):]

			if len(s) == 0 || s[0].kind != tokenParensClose {
				return false, errors.New("incomplete when, wanted )")
			}
			s = s[1:]
			s = s[consumeSpace(s):]

			if !compare(a.v, c, b.v) {
				return false, nil
			}
		} else {
			i, a, _, err := parseNum(s, 0)
			if err != nil {
				return false, fmt.Errorf("incomplete when, wanted a=num: %s", err)
			}
			s = s[i:]
			s = s[consumeSpace(s):]

			i, c, err := parseComp(s, 0)
			if err != nil {
				return false, fmt.Errorf("unexpected when comp: %s", err)
			}
			s = s[i:]
			s = s[consumeSpace(s):]

			i, b, _, err := parseNum(s, 0)
			if err != nil {
				return false, fmt.Errorf("incomplete when, wanted b=num: %s", err)
			}
			s = s[i:]
			s = s[consumeSpace(s):]

			if len(s) == 0 || s[0].kind != tokenParensClose {
				return false, errors.New("incomplete when, wanted )")
			}
			s = s[1:]
			s = s[consumeSpace(s):]

			if !compare(a, c, b) {
				return false, nil
			}
		}

		if len(s) > 0 && s[0].kind == tokenIdentifier && s[0].v == "and" {
			s = s[1:]
			s = s[consumeSpace(s):]
		}
	}
	return true, nil
}

func parseArgs(s tokens, i int) (int, []tokens, error) {
	return parseParams(s, i, false)
}

func parseParams(s tokens, i int, semiOK bool) (int, []tokens, error) {
	if len(s) < i+2 || s[i].kind != tokenParensOpen {
		return i, nil, errors.New("expected args start")
	}
	args := make([]tokens, 0)
	parensLevel := 0
	stringLevel := 0
	start := i + 1
	j := i
	for ; j < len(s); j++ {
		if stringLevel != 0 && s[j].kind != tokenSingleQuote {
			continue
		}
		switch s[j].kind {
		case tokenParensOpen:
			parensLevel++
		case tokenParensClose:
			parensLevel--
		case tokenSingleQuote:
			if stringLevel == 0 {
				stringLevel = 1
			} else {
				stringLevel = 0
			}
		case tokenComma, tokenSemi:
			if !semiOK && s[j].kind == tokenSemi {
				continue
			}
			if parensLevel == 1 {
				args = append(args, trimSpace(s[start:j]))
				start = j + 1
			}
		}
		if parensLevel == 0 {
			break
		}
	}
	if parensLevel > 0 {
		return i, nil, errors.New("expected closing ) after fn")
	}
	last := trimSpace(s[start:j])
	if len(last) > 0 {
		args = append(args, last)
	}
	return j + 1, args, nil
}

func (n *node) evalMixin(s tokens, cc [][]node, pv []map[string]tokens) ([]node, error) {
	var args []tokens
	var name string
	if j := index(s, tokenParensOpen); j != -1 {
		if s[len(s)-1].kind != tokenParensClose {
			return nil, errors.New("expected mixin invocation to end in )")
		}
		name = s[:j].String()
		var err error
		_, args, err = parseParams(s, j, true)
		if err != nil {
			return nil, fmt.Errorf("mixin invocation %q args: %s", name, err)
		}
	} else {
		name = s.String()
	}
	for i, arg := range args {
		args[i] = n.evalDirective(arg, pv)
	}
	var nodes []node

	for _, mixins := range n.mixins {
	nextMixin:
		for _, m := range mixins[name] {
			n1 := m
			if len(n1.matcher) > 0 {
				_, params, err := parseParams(n1.matcher, 0, true)
				if err != nil {
					return nil, fmt.Errorf("mixin %q args: %s", name, err)
				}
				vars := make(map[string]tokens, len(params))
				for i, param := range params {
					if param[0].kind == tokenAt {
						pNameRaw, v, _ := cut(param, tokenColon)
						pName := trimSpace(pNameRaw).String()
						if len(args) > i {
							v = args[i]
						} else {
							v = trimSpace(v)
							if len(v) == 0 {
								continue nextMixin
							}
						}
						for _, arg := range args {
							named, namedVal, ok := cut(arg, tokenColon)
							if ok && named.String() == pName {
								v = trimSpace(namedVal)
								break
							}
						}
						vars[pName[1:]] = v
					} else if param.String() != args[i].String() {
						continue nextMixin
					}
				}
				n1.paramVars = vars
			}
			n1.matcher = nil
			nodes = append(nodes, n1)
		}
		if len(nodes) != 0 {
			return nodes, nil
		}
	}
	if len(args) == 0 {
		for _, children := range cc {
			for _, m := range children {
				if m.matcher.String() != name {
					continue
				}
				n1 := m
				n1.matcher = nil
				nodes = append(nodes, n1)
			}
			if len(nodes) != 0 {
				return nodes, nil
			}
		}

	}
	return nil, fmt.Errorf("mixin %q is unknown", name)
}

func evalStringTemplate(s tokens) tokens {
	before, inner, after := evalStringTemplateOnce(s)
	if len(inner) == 0 && len(after) == 0 {
		return before
	}
	s = make(tokens, 0, len(before)+len(inner)+len(after))
	s = append(s, before...)
	s = append(s, inner...)
	for len(after) > 0 {
		if len(s) > 0 && s[len(s)-1].kind != space &&
			len(after) > 0 && after[0].kind == space {
			s = append(s, after[0])
			after = after[1:]
		}
		before, inner, after = evalStringTemplateOnce(
			evalMath(after),
		)
		s = append(s, before...)
		s = append(s, inner...)
	}
	return s
}

func evalStringTemplateOnce(s tokens) (tokens, tokens, tokens) {
	if len(s) < 3 {
		return s, nil, nil
	}
	idx := index(s, tokenTilde)
	if idx == -1 || idx == len(s)-1 || s[idx+1].kind != tokenSingleQuote {
		return s, nil, nil
	}
	idxEnd := index(s[idx+2:], tokenSingleQuote)
	if idxEnd == -1 {
		return s, nil, nil
	}
	idxEnd += idx + 2
	if idx == 0 && idxEnd == len(s)-1 {
		return s[2 : len(s)-1], nil, nil
	}
	return s[:idx], s[idx+2 : idxEnd], s[idxEnd+1:]
}

func evalStatic(s tokens) tokens {
	s = trimSpace(s)
	s = evalMath(s)
	// TODO: propagate error
	s, _ = evalColor(s)
	s = evalStringTemplate(s)
	return s
}

func (n *node) evalDirective(s tokens, pv []map[string]tokens) tokens {
	if isConstant(s) {
		return evalStringTemplate(s)
	}
	s = n.evalVars(s, pv)
	s = n.evalPaths(s)
	s = evalStatic(s)
	return s
}

func (n *node) evalPaths(s tokens) tokens {
	if len(s) < 3 {
		return s
	}
	var out tokens
	start := 0
	for i := 0; i < len(s)-3; i++ {
		if s[i].kind != tokenIdentifier || s[i].v != "url" {
			continue
		}
		i++
		if s[i].kind != tokenParensOpen {
			continue
		}
		i++
		j := index(s[i:], tokenParensClose)
		if j == -1 {
			continue
		}
		j += i
		if s[i].kind == tokenSingleQuote || s[i].kind == tokenDoubleQuote {
			i++
			j--
		}
		p := path.Join(path.Dir(s[i].f), s[i:j].String())
		out = append(out, s[start:i]...)
		out = append(out, token{kind: tokenIdentifier, v: p})
		start = j
	}
	if start == 0 {
		return s
	}
	out = append(out, s[start:]...)
	return out
}

func (n *node) evalVars(s tokens, pv []map[string]tokens) tokens {
	done := true
	for _, t := range s {
		if t.kind == tokenAt {
			done = false
			break
		}
	}
	if done {
		return s
	}
	s = append(tokens{}, s...)
	for i := 1; i < len(s); i++ {
		if s[i-1].kind == tokenAt && s[i].kind == tokenIdentifier {
			v := n.evalVar(s[i].v, pv)
			s[i-1], s[i] = token{}, token{}
			switch len(v) {
			case 0:
			case 1, 2:
				copy(s[i-1:], v)
			default:
				s = append(append(append(
					make(tokens, 0, len(s)+len(v)),
					s[:i-1]...),
					v...),
					s[i+1:]...)
			}
		}
	}
	for i := 0; i < len(s)-3; i++ {
		if s[i].kind == tokenAt &&
			s[i+1].kind == tokenCurlyOpen &&
			s[i+2].kind == tokenIdentifier &&
			s[i+3].kind == tokenCurlyClose {
			v := n.evalVar(s[i+2].v, pv)
			s[i], s[i+1], s[i+2], s[i+3] = token{}, token{}, token{}, token{}
			switch len(v) {
			case 0:
			case 1, 2, 3, 4:
				copy(s[i:], v)
			default:
				s = append(append(append(
					make(tokens, 0, len(s)+len(v)),
					s[:i]...),
					v...),
					s[i+4:]...)
			}
		}
	}
	return trimSpace(s)
}

func (n *node) evalVar(name string, pv []map[string]tokens) tokens {
	for _, source := range [][]map[string]tokens{pv, n.vars} {
	nextSource:
		for _, vars := range source {
			s, ok := vars[name]
			if !ok {
				continue
			}
			if isConstant(s) {
				return evalStringTemplate(s)
			}

			s = append(tokens{}, s...)
			for i := 1; i < len(s); i++ {
				if s[i-1].kind == tokenAt && s[i].kind == tokenIdentifier {
					if s[i].v == name {
						continue nextSource
					}
					v := n.evalVar(s[i].v, pv)
					switch len(v) {
					case 0:
						s[i-1], s[i] = token{}, token{}
					case 1:
						s[i-1], s[i] = token{}, v[0]
					case 2:
						s[i-1], s[i] = v[0], v[1]
					default:
						s = append(append(append(
							make(tokens, 0, len(s)+len(v)),
							s[:i-1]...),
							v...),
							s[i+1:]...)
					}
				}
			}
			s = evalStatic(s)
			vars[name] = s
			return s
		}
	}
	return tokens{{kind: tokenAt, v: "@"}, {kind: tokenIdentifier, v: name}}
}

func (n *node) print(w *strings.Builder, p tokens, m, opened matchers, cc [][]node, pv []map[string]tokens, addSpace bool) (bool, []tokens, error) {
	if len(n.directives) == 0 &&
		len(n.children) == 0 &&
		!isKeyframes(p) {
		return addSpace, opened, nil
	}
	if n.paramVars != nil {
		pv = append([]map[string]tokens{n.paramVars}, pv...)
	}
	if n.children != nil {
		cc = append(cc, n.children)
	}
	if ok, err := n.evalWhen(pv); err != nil {
		return false, opened, err
	} else if !ok {
		return addSpace, opened, nil
	}
	nest, m := n.evalMatcher(pv, p, m)
	if len(nest) > 0 {
		if len(opened) > 0 {
			w.WriteString(" }")
			opened = nil
		}
		if addSpace {
			w.WriteString(" ")
		}
		addSpace = true
		nest.WriteString(w)
		w.WriteString(" {")
		p = nest
	}
	directives := n.directives
	if len(n.directives) == 1 && n.directives[0].name == "each" {
		src, err := n.evalMixin(n.directives[0].value, cc, pv)
		if err != nil {
			return addSpace, opened, err
		}
		directives = make([]directive, 0, len(src[0].directives))
		for _, d := range src[0].directives {
			v := make(tokens, 0, len(d.value)+5)
			v = append(v,
				token{kind: tokenIdentifier, v: ".each"},
				token{kind: tokenParensOpen, v: "("},
				token{kind: tokenIdentifier, v: d.name},
				token{kind: tokenComma, v: ","},
			)
			v = append(v, d.value...)
			v = append(v, token{kind: tokenParensClose, v: ")"})
			directives = append(directives, directive{value: v})
		}
	}
	for _, d := range directives {
		if d.name == "" {
			mixins, err := n.evalMixin(d.value, cc, pv)
			if err != nil {
				return addSpace, opened, err
			}
			for _, child := range mixins {
				if addSpace, opened, err = child.print(w, p, m, opened, cc, pv, addSpace); err != nil {
					return addSpace, opened, err
				}
			}
			continue
		}
		if len(m) > 0 && !opened.Eq(m) {
			if len(opened) > 0 {
				w.WriteString(" }")
			}
			opened = m
			if addSpace {
				w.WriteString(" ")
			}
			addSpace = true
			for i, t := range m {
				if i > 0 {
					w.WriteString(",")
				}
				t.WriteString(w)
			}
			w.WriteString(" {")
		}
		if addSpace {
			w.WriteString(" ")
		}
		w.WriteString(d.name)
		if d.name == "@charset" {
			w.WriteString(" ")
		} else {
			w.WriteString(": ")
		}
		n.evalDirective(d.value, pv).WriteString(w)
		w.WriteString(";")
		addSpace = true
	}
	for _, child := range n.children {
		var err error
		addSpace, opened, err = child.print(w, p, m, opened, cc, pv, addSpace)
		if err != nil {
			return addSpace, opened, err
		}
	}
	if len(nest) > 0 {
		if len(opened) > 0 {
			w.WriteString(" }")
			opened = nil
		}
		w.WriteString(" }")
	}
	return addSpace, opened, nil
}

type parser struct {
	read func(name string) ([]byte, error)
	root *node
}

func (p *parser) parse(f string) error {
	p.root = &node{
		vars:   []map[string]tokens{{}},
		mixins: []map[string][]node{{}},
	}
	return p.root.parse(p.read, f)
}

func (n *node) parse(read func(name string) ([]byte, error), f string) error {
	n.imports = append(n.imports, f)

	blob, err := read(f)
	if err != nil {
		return err
	}
	s := string(blob)
	tt := tokenize(s, f)
	i, err := n.consume(f, read, tt, 0)
	if err == nil && i != len(tt) {
		err = errors.New("should consume in full")
	}
	if err != nil {
		end := len(s)
		if len(tt) > i+2 && tt[i].kind == tokenAt && tt[i+1].kind == tokenIdentifier {
			idx := index(tt[i:], tokenSemi)
			if idx != -1 {
				end = int(tt[i+idx].start) + 1
			}
		}
		fmt.Printf("consumed %q until t=%d p=%d: %q: %s\n", f, i, tt[i].start, err, s[tt[i].start:end])
		return err
	}
	return nil
}

func (n *node) collectImports(c []string) []string {
	c = append(c, n.imports...)
	for _, child := range n.children {
		c = child.collectImports(c)
	}
	return c
}

func (p *parser) getImports() []string {
	return p.root.collectImports(nil)
}

func (p *parser) print() (string, error) {
	w := strings.Builder{}
	_, opened, err := p.root.print(&w, nil, nil, nil, nil, nil, false)
	if err != nil {
		return "", err
	}
	if len(opened) > 0 {
		w.WriteString(" }")
	}
	return strings.TrimLeft(w.String(), " "), nil
}

func (n *node) printRaw(w *strings.Builder, indent string) error {
	indent1 := indent
	if len(n.matcher) > 0 {
		w.WriteString(indent)
		n.matcher.WriteString(w)
		w.WriteString(" {")
		indent1 = indent + "  "
	}
	for k, v := range n.vars[0] {
		w.WriteString(indent1)
		w.WriteString("@")
		w.WriteString(k)
		w.WriteString(": ")
		v.WriteString(w)
		w.WriteString(";")
	}
	for _, d := range n.directives {
		w.WriteString(indent1)
		if d.name != "" {
			w.WriteString(d.name)
			w.WriteString(": ")
		}
		d.value.WriteString(w)
		w.WriteString(";")
	}
	for _, child := range n.children {
		if err := child.printRaw(w, indent1); err != nil {
			return err
		}
	}
	indent2 := indent1 + "  "
	for name, nodes := range n.mixins[0] {
		for _, n1 := range nodes {
			w.WriteString(indent1)
			w.WriteString(name)
			w.WriteString("(")
			n1.matcher.WriteString(w)
			w.WriteString(") ")
			if len(n1.when) > 0 {
				w.WriteString("when ")
				n1.when.WriteString(w)
				w.WriteString(" ")
			}
			w.WriteString("{")
			n1.matcher = nil
			n1.when = nil
			if err := n1.printRaw(w, indent2); err != nil {
				return err
			}
			w.WriteString(indent1)
			w.WriteString("}")
		}
	}
	if len(n.matcher) > 0 {
		w.WriteString(indent)
		w.WriteString("}")
	}
	return nil
}

func (p *parser) printRaw() (string, error) {
	w := strings.Builder{}
	if err := p.root.printRaw(&w, "\n~"); err != nil {
		return "", err
	}
	return strings.TrimSpace(w.String()), nil
}
