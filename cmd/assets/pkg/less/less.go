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
	"regexp"
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
	return p.print(), p.getImports(), nil
}

type directive struct {
	name  string
	value string
}

type node struct {
	f          string
	matcher    string
	when       string
	directives []directive
	children   []node
	imports    []string
	vars       []map[string]string
	paramVars  map[string]string
	mixins     []map[string][]node
}

var importPrefixes = []string{"@import '", "@import (less) '"}

func isAtRule(s string) bool {
	switch s {
	case
		"@charset",
		"@font-face",
		"@keyframes", "@-moz-keyframes", "@-webkit-keyframes",
		"@media":
		return true
	default:
		return false
	}
}

func (n *node) consume(read func(name string) ([]byte, error), s string) (int, error) {
nextChar:
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ', '\n':
			continue
		case '}':
			return i + 1, nil
		case '/':
			if len(s) > i && s[i+1] == '/' {
				end := strings.IndexRune(s[i:], '\n')
				i += end
				continue
			}
			if len(s) > i && s[i+1] == '*' {
				end := strings.Index(s[i:], "*/")
				i += end + 1
				continue
			}
			return i, errors.New("unexpected /")
		case '@':
			for _, importPrefix := range importPrefixes {
				if strings.HasPrefix(s[i:], importPrefix) {
					i += len(importPrefix)
					end := strings.Index(s[i:], "';")
					next := path.Join(path.Dir(n.f), s[i:i+end])
					if err := n.parse(read, next); err != nil {
						return i, err
					}
					i += end + 1
					continue nextChar
				}
			}
			nextWord, _, _ := strings.Cut(s[i:], " ")
			if nextWord == "@charset" {
				i += strings.IndexRune(s[i:], ';')
				continue
			}
			if !isAtRule(nextWord) {
				// variable
				nameEnd := strings.IndexRune(s[i:], ':')
				name := strings.TrimSpace(s[i+1 : i+nameEnd])
				valueEnd := strings.IndexRune(s[i+nameEnd:], ';')
				value := strings.TrimSpace(s[i+nameEnd+1 : i+nameEnd+valueEnd])
				n.vars[0][name] = value
				i += nameEnd + valueEnd
				continue
			}
		}

		dot := strings.IndexRune(s[i:], '.')
		open := strings.IndexRune(s[i:], '{')
		for open > 0 && s[i+open-1] == '@' {
			// .mixin(@var) { .@{var} {} }
			next := strings.IndexRune(s[i+open+1:], '{')
			if next == -1 {
				open = -1
			} else {
				open += next + 1
			}
		}
		semi := strings.IndexRune(s[i:], ';')
		parensOpen := strings.IndexRune(s[i:], '(')
		parensClose := strings.IndexRune(s[i:], ')')
		colon := strings.IndexRune(s[i:i+semi+1], ':')
		gt := strings.IndexRune(s[i:], '>')
		if open != -1 && len(s) > open && (semi == -1 || open < semi || (dot == 0 && (colon == -1 || parensOpen < colon) && parensOpen < semi && semi < parensClose && parensClose < open)) {
			n1 := node{
				f:      n.f,
				vars:   append([]map[string]string{{}}, n.vars...),
				mixins: append([]map[string][]node{{}}, n.mixins...),
			}
			n1.matcher, n1.when, _ = strings.Cut(s[i:i+open], " when")
			n1.when = strings.TrimSpace(n1.when)
			n1.matcher = strings.TrimSpace(n1.matcher)
			n1.matcher = strings.ReplaceAll(n1.matcher, "\n", " ")
			n1.matcher = strings.ReplaceAll(n1.matcher, "  ", " ")
			off, err := n1.consume(read, s[i+open+1:])
			if strings.HasPrefix(n1.matcher, ".") && strings.HasSuffix(n1.matcher, ")") {
				name, args, _ := strings.Cut(n1.matcher, "(")
				n1.matcher = strings.TrimSuffix(args, ")")
				n.mixins[0][name] = append(n.mixins[0][name], n1)
				if strings.HasPrefix(n.matcher, "#") {
					chain := n.matcher + " > " + name
					n.mixins[len(n.mixins)-1][chain] = append(
						n.mixins[len(n.mixins)-1][chain], n1,
					)
				}
			} else {
				n.children = append(n.children, n1)
			}
			if err != nil {
				return i, err
			}
			i += open + off
		} else if dot == 0 && parensClose == semi-1 {
			n.directives = append(n.directives, directive{
				value: strings.TrimSpace(s[i : i+semi]),
			})
			i += semi
		} else if dot == 0 && len(s) > i+parensClose && s[i+parensClose+1] == ';' && parensOpen < semi && semi < parensClose && (colon == -1 || (parensOpen < colon && colon < parensClose)) {
			n.directives = append(n.directives, directive{
				value: strings.TrimSpace(s[i : i+parensClose+1]),
			})
			i += parensClose + 1
		} else if dot == 0 && s[i+semi-1] == ')' && (colon == -1 || parensOpen < colon) && strings.Count(s[i+parensOpen+1:], "(") == strings.Count(s[i+semi-1:], ")") {
			n.directives = append(n.directives, directive{
				value: strings.TrimSpace(s[i : i+semi]),
			})
			i += semi
		} else if gt != -1 && dot != -1 && parensOpen != -1 && parensClose != -1 && semi != -1 && gt < dot && dot < parensOpen && parensOpen < parensClose && parensClose < semi && colon == -1 {
			n.directives = append(n.directives, directive{
				value: strings.TrimSpace(s[i : i+semi]),
			})
			i += semi
		} else {
			if colon == -1 {
				fmt.Println(s[i : i+semi])
				return i, errors.New("expected colon before next semi")
			}
			n.directives = append(n.directives, directive{
				name:  strings.TrimSpace(s[i : i+colon]),
				value: strings.TrimSpace(s[i+colon+1 : i+semi]),
			})
			i += semi
		}
	}
	return len(s), nil
}

func isConstant(s string) bool {
	if len(s) == 0 {
		return false
	}
	if strings.ContainsAny(s, "@+-*/") {
		return false
	}
	if fn, _, ok := strings.Cut(s, "("); ok {
		switch fn {
		case "rgb", "rgba", "hsl", "hsla", "hwb":
			return true
		default:
			return false
		}
	}
	return true
}

func (n *node) evalDirectives(pv []map[string]string) {
	for i, d := range n.directives {
		n.directives[i].value = n.evalDirective(d.value, pv)
	}
}

func (n *node) evalMatcher(pv []map[string]string) string {
	if len(n.matcher) == 0 {
		return ""
	}
	s := n.matcher
	for _, nested := range varRegex.FindAllStringSubmatch(s, -1) {
		if isAtRule(nested[0]) {
			continue
		}
		s = strings.ReplaceAll(s, nested[0], n.evalVar(nested[2], pv))
	}
	return s
}

func evalMath(s string) string {
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		s = strings.TrimPrefix(strings.TrimSuffix(s, ")"), "(")
	}
	parts := strings.Fields(s)
	if len(parts) != 3 {
		return s
	}
	a, operator, b := parts[0], parts[1], parts[2]
	aInt, _ := strconv.ParseInt(a, 10, 64)
	bInt, _ := strconv.ParseInt(b, 10, 64)
	var r int64
	switch operator {
	case "+":
		r = aInt + bInt
	case "-":
		r = aInt - bInt
	case "/":
		r = aInt / bInt
	case "*":
		r = aInt * bInt
	default:
		return s
	}
	return strconv.FormatInt(r, 10)
}

func (n *node) evalWhen(pv []map[string]string) bool {
	if len(n.when) == 0 {
		return true
	}
	s := n.when
	for _, nested := range varRegex.FindAllStringSubmatch(s, -1) {
		if isAtRule(nested[0]) {
			continue
		}
		s = strings.ReplaceAll(s, nested[0], n.evalVar(nested[2], pv))
	}
	for _, condition := range strings.Split(s, "and") {
		condition = strings.TrimSpace(condition)
		condition = strings.TrimPrefix(condition, "(")
		condition = strings.TrimSuffix(condition, ")")
		parts := strings.Fields(condition)
		a, comparator, b := parts[0], parts[1], parts[2]
		aInt, _ := strconv.ParseInt(a, 10, 64)
		bInt, _ := strconv.ParseInt(b, 10, 64)
		switch comparator {
		case "=":
			if a != b {
				return false
			}
		case "!=":
			if a == b {
				return false
			}
		case "<":
			if aInt >= bInt {
				return false
			}
		case ">":
			if aInt <= bInt {
				return false
			}
		case "<=", "=<":
			if aInt > bInt {
				return false
			}
		case ">=", "=>":
			if aInt < bInt {
				return false
			}
		}
	}
	return true
}

func getArgs(s string) []string {
	parts := make([]string, 0)
	l := 0
	start := 0
	for i, r := range s {
		switch r {
		case '(':
			l++
		case ')':
			l--
		case ',', ';':
			if l == 0 {
				parts = append(parts, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	last := strings.TrimSpace(s[start:])
	if len(last) > 0 {
		parts = append(parts, last)
	}
	return parts
}

var varRegex = regexp.MustCompile(`(\${)?@{?([\w-]+)}?`)

func (n *node) evalMixin(s string, pv []map[string]string) []node {
	name, argsRaw, _ := strings.Cut(s, "(")
	args := getArgs(strings.TrimSuffix(argsRaw, ")"))
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
				vars := make(map[string]string, len(params))
				for i, param := range params {
					if strings.HasPrefix(param, "@") {
						pName, defaultValue, _ := strings.Cut(param, ":")
						v := strings.TrimSpace(defaultValue)
						if len(args) > i {
							v = args[i]
						}
						for _, arg := range args {
							named, namedVal, ok := strings.Cut(arg, ":")
							if ok && named == pName {
								v = strings.TrimSpace(namedVal)
								break
							}
						}
						vars[pName[1:]] = v
					} else if param != args[i] {
						continue nextMixin
					}
				}
				n1.paramVars = vars
			}
			n1.matcher = ""
			nodes = append(nodes, n1)
		}
		if len(nodes) != 0 {
			return nodes
		}
	}
	panic(fmt.Sprintf("mixin %q is unknown", name))
}

func (n *node) evalDirective(s string, pv []map[string]string) string {
	if isConstant(s) {
		return s
	}
	for _, nested := range varRegex.FindAllStringSubmatch(s, -1) {
		s = strings.ReplaceAll(s, nested[0], n.evalVar(nested[2], pv))
	}
	s = evalMath(s)
	return s
}

func (n *node) evalVar(name string, pv []map[string]string) string {
	for _, source := range [][]map[string]string{pv, n.vars} {
	nextSource:
		for _, vars := range source {
			s, ok := vars[name]
			if !ok {
				continue
			}
			if isConstant(s) {
				return s
			}
			for _, nested := range varRegex.FindAllStringSubmatch(s, -1) {
				if nested[2] == name {
					continue nextSource
				}
				s = strings.ReplaceAll(s, nested[0], n.evalVar(nested[2], pv))
			}
			s = evalMath(s)
			vars[name] = s
			return s
		}
	}
	return "@" + name
}

func (n *node) print(w *strings.Builder, pv []map[string]string, addSpace bool) bool {
	if len(n.directives) == 0 && len(n.children) == 0 && !strings.HasSuffix(n.matcher, "%") {
		return addSpace
	}
	if n.paramVars != nil {
		pv = append([]map[string]string{n.paramVars}, pv...)
	}
	if !n.evalWhen(pv) {
		return addSpace
	}
	matcher := n.evalMatcher(pv)

	if matcher != "" {
		if addSpace {
			w.WriteString(" ")
		}
		addSpace = true
		w.WriteString(matcher)
		w.WriteString(" {")
	}
	for _, d := range n.directives {
		if d.name == "" {
			for _, child := range n.evalMixin(d.value, pv) {
				addSpace = child.print(w, pv, addSpace)
			}
			continue
		}
		if addSpace {
			w.WriteString(" ")
		}
		addSpace = true
		w.WriteString(d.name)
		w.WriteString(": ")
		w.WriteString(n.evalDirective(d.value, pv))
		w.WriteString(";")
	}
	for _, child := range n.children {
		addSpace = child.print(w, pv, addSpace)
	}
	if matcher != "" {
		w.WriteString(" }")
	}
	return addSpace
}

type parser struct {
	read func(name string) ([]byte, error)
	root *node
}

func (p *parser) parse(f string) error {
	p.root = &node{
		f:      f,
		vars:   []map[string]string{{}},
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
	consumed, err := n.consume(read, string(blob))
	if err != nil {
		return err
	}
	if consumed != len(blob) {
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

func (p *parser) print() string {
	w := strings.Builder{}
	p.root.print(&w, nil, false)
	return strings.TrimLeft(w.String(), " ")
}

func (n *node) printMixins(w *strings.Builder) {
	if len(n.mixins[0]) == 0 {
		return
	}
	if len(n.matcher) > 0 {
		w.WriteString(n.matcher)
		w.WriteString(" {")
	}
	for name, nodes := range n.mixins[0] {
		for _, n1 := range nodes {
			w.WriteString(" ")
			w.WriteString(name)
			w.WriteString("(")
			w.WriteString(n1.matcher)
			w.WriteString(") ")
			if n1.when != "" {
				w.WriteString("when ")
				w.WriteString(n1.when)
				w.WriteString(" ")
			}
			w.WriteString("{ ")
			n1.matcher = ""
			n1.when = ""
			n1.vars = nil
			n1.print(w, nil, false)
			n1.printMixins(w)
			w.WriteString(" }")
		}
	}
	if len(n.matcher) > 0 {
		w.WriteString(" }")
	}
}

func (p *parser) printMixins() string {
	w := strings.Builder{}
	p.root.printMixins(&w)
	return strings.TrimLeft(w.String(), " ")
}
