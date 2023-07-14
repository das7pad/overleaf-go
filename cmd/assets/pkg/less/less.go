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
	"strconv"
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
		fmt.Println(p.printMixins())
		fmt.Println(p.print())
		return "", p.getImports(), err
	}
	fmt.Println(p.printMixins())
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
	f          string
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
	return -1, errors.New(fmt.Sprintf("%s not found", needle))
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

var whenToken = token{kind: tokenIdentifier, v: "when"}

func (n *node) consume(read func(name string) ([]byte, error), tt tokens) (int, error) {
doneParsing:
	for i := 0; i < len(tt); i++ {
	topLevelToken:
		switch tt[i].kind {
		case space, tokenNewline:
			continue
		case tokenCurlyClose:
			return i + 1, nil
		case tokenSlash:
			if len(tt) > i+1 {
				t1 := tt[i+1]
				switch t1.kind {
				case tokenSlash:
					j, err := consumeUntil(tt[i+1:], tokenNewline)
					if err != nil {
						return i, err
					}
					i += j + 1
				case tokenStar:
					j, err := consumeUntil(tt[i+1:], tokenStar, tokenSlash)
					if err != nil {
						return i, err
					}
					i += j + 2
				}
				continue
			}
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
								return int(tt[j].start), err
							}
							next := path.Join(
								path.Dir(n.f), tt[j:j+k].String(),
							)
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
									value: tt[j+1 : j+2],
								},
							}, n.directives...)
							j += 3
							j += consumeSpace(tt[j:])
							if len(tt) == j+1 || tt[j+1].kind != tokenSemi {
								return i, errors.New("expected semi after charset")
							}
							i = j + 1
							continue
						}
					case
						"font-face",
						"keyframes", "-moz-keyframes", "-webkit-keyframes",
						"media":
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
							n.vars[0][t1.v] = trimSpace(tt[j : j+k])
							i = j + k
							continue
						}
					}
				}
			}
			return i, errors.New("unexpected '@'")
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
			case tokenIdentifier:
				if t2.v == "when" {
					when = j
				}
			}
		}
		if t2.kind == tokenSemi {
			if colon == -1 {
				return i, errors.New("expected : before semi")
			}
			n.directives = append(n.directives, directive{
				name:  tt[i : i+colon].String(),
				value: trimSpace(tt[i+colon+1 : i+j]),
			})
			i += j
			continue
		}
		if t2.kind == tokenParensOpen {
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
							value: trimSpace(tt[i : end+1]),
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
		n1 := node{
			f:      n.f,
			vars:   append([]map[string]tokens{{}}, n.vars...),
			mixins: append([]map[string][]node{{}}, n.mixins...),
		}
		n1.matcher, n1.when, _ = cutToken(tt[i:i+j], whenToken)
		n1.matcher = trimSpace(n1.matcher)
		n1.when = trimSpace(n1.when)
		off, err := n1.consume(read, tt[i+j+1:])
		switch t2.kind {
		case tokenParensOpen:
			nameRaw, args, _ := cut(n1.matcher, tokenParensOpen)
			if len(args) == 0 {
				return i, errors.New("expected args")
			}
			n1.matcher = args[:len(args)-1]
			name := trimSpace(nameRaw).String()
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
		i += j + off
	}
	return len(tt), nil
}

func isConstant(s tokens) bool {
	if len(s) == 0 {
		return false
	}
	isCalc := (len(s) > 2 &&
		s[0].kind == tokenIdentifier &&
		s[0].v == "calc" &&
		s[1].kind == tokenParensOpen) || (len(s) > 4 &&
		s[0].kind == tokenTilde &&
		s[1].kind == tokenSingleQuote &&
		s[2].kind == tokenIdentifier &&
		s[2].v == "calc" &&
		s[3].kind == tokenParensOpen)
	for i, t := range s {
		switch t.kind {
		case tokenAt:
			return false
		case tokenPlus, tokenMinus, tokenStar, tokenSlash:
			if !isCalc {
				return false
			}
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

func (n *node) evalMatcher(pv []map[string]tokens) tokens {
	return n.evalVars(n.matcher, pv)
}

func evalMath(s tokens) tokens {
	i := 0
	i += consumeSpace(s[i:])
	if len(s) > i && s[i].kind == tokenParensOpen {
		i += 1
		i += consumeSpace(s[i:])
	}
	if len(s) == i || s[i].kind != tokenNum {
		return s
	}
	a := s[i]
	i += 1
	i += consumeSpace(s[i:])
	if len(s) == i {
		return s
	}
	operator := s[i]
	i += 1
	i += consumeSpace(s[i:])
	if len(s) == i || s[i].kind != tokenNum {
		return s
	}
	b := s[i]
	i += 1
	i += consumeSpace(s[i:])
	if len(s) > i && s[i].kind == tokenParensClose {
		i += 1
		i += consumeSpace(s[i:])
	}
	if len(s) != i {
		return s
	}
	aInt, _ := strconv.ParseInt(a.v, 10, 64)
	bInt, _ := strconv.ParseInt(b.v, 10, 64)
	var r int64
	switch operator.kind {
	case tokenPlus:
		r = aInt + bInt
	case tokenMinus:
		r = aInt - bInt
	case tokenSlash:
		r = aInt / bInt
	case tokenStar:
		r = aInt * bInt
	default:
		return s
	}
	return tokens{{kind: tokenNum, v: strconv.FormatInt(r, 10)}}
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

		if len(s) == 0 || !(s[0].kind == tokenNum || s[0].kind == tokenIdentifier) {
			return false, errors.New("incomplete when, wanted a=num/identifier")
		}
		a := s[0]
		s = s[1:]
		s = s[consumeSpace(s):]

		if len(s) < 2 {
			return false, errors.New("incomplete when, missing comp or b")
		}

		var c kind
		switch s[0].kind {
		case tokenEq:
			c = compEq
		case tokenExclamation:
			c = tokenExclamation
		case tokenLt:
			c = compLt
		case tokenGt:
			c = compGt
		default:
			return false, errors.New("incomplete when, wanted comp")
		}
		s = s[1:]

		switch s[0].kind {
		case tokenEq:
			switch c {
			case compLt:
				c = compLte
			case compGt:
				c = compGte
			case tokenExclamation:
				c = compNEq
			default:
				return false, errors.New("incomplete when, bad eq comp")
			}
			s = s[1:]
		case tokenLt:
			if c == compEq {
				c = compLte
			} else {
				return false, errors.New("incomplete when, bad lt comp")
			}
			s = s[1:]
		case tokenGt:
			if c == compEq {
				c = compGte
			} else {
				return false, errors.New("incomplete when, bad gt comp")
			}
			s = s[1:]
		default:
			if c == tokenExclamation {
				return false, errors.New("incomplete when, bad ! comp")
			}
		}
		s = s[consumeSpace(s):]

		if len(s) == 0 || !(s[0].kind == tokenNum || s[0].kind == tokenIdentifier) {
			return false, errors.New("incomplete when, wanted b=num/identifier")
		}
		b := s[0]
		s = s[1:]
		s = s[consumeSpace(s):]

		if len(s) == 0 || s[0].kind != tokenParensClose {
			return false, errors.New("incomplete when, wanted )")
		}
		s = s[1:]
		s = s[consumeSpace(s):]

		aInt, _ := strconv.ParseInt(a.v, 10, 64)
		bInt, _ := strconv.ParseInt(b.v, 10, 64)
		switch c {
		case compEq:
			if a.v != b.v {
				return false, nil
			}
		case compNEq:
			if a.v == b.v {
				return false, nil
			}
		case compLt:
			if aInt >= bInt {
				return false, nil
			}
		case compGt:
			if aInt <= bInt {
				return false, nil
			}
		case compLte:
			if aInt > bInt {
				return false, nil
			}
		case compGte:
			if aInt < bInt {
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

func getArgs(s tokens) []tokens {
	parts := make([]tokens, 0)
	parensLevel := 0
	stringLevel := 0
	start := 0
	for i, r := range s {
		switch r.kind {
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
			if parensLevel == 0 && stringLevel == 0 {
				parts = append(parts, trimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	last := trimSpace(s[start:])
	if len(last) > 0 {
		parts = append(parts, last)
	}
	return parts
}

func (n *node) evalMixin(s tokens, pv []map[string]tokens) ([]node, error) {
	j, err := consumeUntil(s, tokenParensOpen)
	if err != nil {
		return nil, err
	}
	if s[len(s)-1].kind != tokenParensClose {
		return nil, errors.New("expected mixin invocation to end in )")
	}
	name := s[:j].String()
	args := getArgs(s[j+1 : len(s)-1])
	for i, arg := range args {
		args[i] = n.evalDirective(arg, pv)
	}
	var nodes []node

	for _, mixins := range n.mixins {
	nextMixin:
		for _, m := range mixins[name] {
			n1 := m
			params := getArgs(n1.matcher)
			if len(params) > 0 {
				vars := make(map[string]tokens, len(params))
				for i, param := range params {
					if param[0].kind == tokenAt {
						pNameRaw, v, _ := cut(param, tokenColon)
						pName := trimSpace(pNameRaw).String()
						v = trimSpace(v)
						if len(args) > i {
							v = args[i]
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
	panic(fmt.Sprintf("mixin %q is unknown", name))
}

func removeStringTemplate(s tokens) tokens {
	s = trimSpace(s)
	if len(s) < 3 {
		return s
	}
	if s[0].kind == tokenTilde &&
		s[1].kind == tokenSingleQuote &&
		s[len(s)-1].kind == tokenSingleQuote {
		return s[2 : len(s)-1]
	}
	return s
}

func evalStatic(s tokens) tokens {
	s = trimSpace(s)
	s = evalMath(s)
	s = removeStringTemplate(s)
	return s
}

func (n *node) evalDirective(s tokens, pv []map[string]tokens) tokens {
	if isConstant(s) {
		return s
	}
	s = n.evalVars(s, pv)
	s = evalStatic(s)
	return s
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
		// TODO: ${@foo} ?
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
				return s
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

func (n *node) print(w *strings.Builder, pv []map[string]tokens, addSpace bool) (bool, error) {
	if len(n.directives) == 0 &&
		len(n.children) == 0 &&
		!(len(n.matcher) > 0 &&
			n.matcher[len(n.matcher)-1].kind == tokenPercent) {
		return addSpace, nil
	}
	if n.paramVars != nil {
		pv = append([]map[string]tokens{n.paramVars}, pv...)
	}
	if ok, err := n.evalWhen(pv); err != nil {
		return false, err
	} else if !ok {
		return addSpace, nil
	}
	matcher := n.evalMatcher(pv)

	if len(matcher) > 0 {
		if addSpace {
			w.WriteString(" ")
		}
		addSpace = true
		matcher.WriteString(w)
		w.WriteString(" {")
	}
	for _, d := range n.directives {
		if d.name == "" {
			mixins, err := n.evalMixin(d.value, pv)
			if err != nil {
				return false, err
			}
			for _, child := range mixins {
				if addSpace, err = child.print(w, pv, addSpace); err != nil {
					return false, err
				}
			}
			continue
		}
		if addSpace {
			w.WriteString(" ")
		}
		addSpace = true
		w.WriteString(d.name)
		if d.name == "@charset" {
			w.WriteString(" ")
		} else {
			w.WriteString(": ")
		}
		n.evalDirective(d.value, pv).WriteString(w)
		w.WriteString(";")
	}
	for _, child := range n.children {
		var err error
		addSpace, err = child.print(w, pv, addSpace)
		if err != nil {
			return false, err
		}
	}
	if len(matcher) > 0 {
		w.WriteString(" }")
	}
	return addSpace, nil
}

type parser struct {
	read func(name string) ([]byte, error)
	root *node
}

func (p *parser) parse(f string) error {
	p.root = &node{
		f:      f,
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
	fmt.Println(f, []token(tokenize(string(blob))))
	s := string(blob)
	tt := tokenize(s)
	i, err := n.consume(read, tt)
	if err != nil {
		fmt.Printf("consumed until %d: %s\n", i, s[tt[i].start:])
		return err
	}
	if i != len(tt) {
		return errors.New("should consume in full")
	}
	return err
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
	_, err := p.root.print(&w, nil, false)
	if err != nil {
		return "", err
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

func (p *parser) printMixins() (string, error) {
	w := strings.Builder{}
	if err := p.root.printRaw(&w, "\n"); err != nil {
		return "", err
	}
	return strings.TrimSpace(w.String()), nil
}
