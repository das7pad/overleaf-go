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
		fmt.Println(p.print())
		return "", p.getImports(), err
	}
	p.eval()
	return p.print(), p.getImports(), nil
}

type directive struct {
	name  string
	value string
}

type node struct {
	f          string
	matcher    string
	directives []directive
	children   []*node
	imports    []string
	vars       []map[string]string
	mixins     map[string][]node
}

var importPrefixes = []string{"@import '", "@import (less) '"}

func isAtRule(s string) bool {
	switch s {
	case
		"@media",
		"@font-face",
		"@keyframes", "@-moz-keyframes", "@-webkit-keyframes":
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

		open := strings.IndexRune(s[i:], '{')
		semi := strings.IndexRune(s[i:], ';')
		parensClose := strings.IndexRune(s[i:], ')')
		if open != -1 && len(s) > open && (semi == -1 || open < semi) {
			n1 := node{
				f:      n.f,
				vars:   append([]map[string]string{{}}, n.vars...),
				mixins: n.mixins,
			}
			n1.matcher = strings.TrimSpace(s[i : i+open])
			off, err := n1.consume(read, s[i+open+1:])
			if strings.HasPrefix(n1.matcher, ".") && strings.HasSuffix(n1.matcher, ")") {
				name, args, _ := strings.Cut(n1.matcher, "(")
				n1.matcher = strings.TrimSuffix(args, ")")
				n.mixins[name] = append(n.mixins[name], n1)
			} else {
				n.children = append(n.children, &n1)
			}
			if err != nil {
				return i, err
			}
			i += open + off
		} else if parensClose == semi-1 {
			n.directives = append(n.directives, directive{
				value: strings.TrimSpace(s[i : i+semi]),
			})
			i += semi
		} else {
			colon := strings.IndexRune(s[i:i+semi], ':')
			if colon == -1 {
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
	if strings.ContainsRune(s, '@') {
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

func (n *node) evalDirectives() {
	// TODO: skip when WHEN=false

	for i, d := range n.directives {
		n.directives[i].value = n.evalDirective(d.value)
	}
	for _, child := range n.children {
		child.evalDirectives()
	}
}

func (n *node) evalMatcher() {
	if len(n.matcher) > 0 {
		s := n.matcher
		for _, nested := range varRegex.FindAllStringSubmatch(s, -1) {
			if isAtRule(nested[0]) {
				continue
			}
			s = strings.ReplaceAll(s, nested[0], n.evalVar(nested[2]))
		}
		n.matcher = s

		// TODO: mixin
		// TODO: flag WHEN
	}
	for _, child := range n.children {
		child.evalMatcher()
	}
}

func getArgs(s string) []string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';'
	})
	out := make([]string, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) > 0 {
			out = append(out, part)
		}
	}
	return out
}

var varRegex = regexp.MustCompile(`(\${)?@([\w-]+)(})?`)

func (n *node) evalDirective(s string) string {
	if isConstant(s) {
		return s
	}
	for _, nested := range varRegex.FindAllStringSubmatch(s, -1) {
		s = strings.ReplaceAll(s, nested[0], n.evalVar(nested[2]))
	}
	if strings.HasPrefix(s, ".") && strings.HasSuffix(s, ")") {
		name, argsRaw, _ := strings.Cut(s, "(")
		args := getArgs(strings.TrimSuffix(argsRaw, ")"))
	nextMixin:
		for _, m := range n.mixins[name] {
			n1 := m
			params := getArgs(n1.matcher)
			if len(params) > 0 {
				vars := make(map[string]string, len(params))
				for i, param := range params {
					if strings.HasPrefix(param, "@") {
						vars[param[1:]] = args[i]
					} else if param != args[i] {
						continue nextMixin
					}
				}
				n1.vars = append([]map[string]string{vars}, m.vars...)
			}
			n1.matcher = ""
			n.children = append(n.children, &n1)
		}
		return ""
	}
	return s
}

func (n *node) evalVar(name string) string {
	for _, vars := range n.vars {
		s, ok := vars[name]
		if !ok {
			continue
		}
		if isConstant(s) {
			return s
		}
		for _, nested := range varRegex.FindAllStringSubmatch(s, -1) {
			s = strings.ReplaceAll(s, nested[0], n.evalVar(nested[2]))
		}
		vars[name] = s
		return s
	}
	return "@" + name
}

func (n *node) print(w *strings.Builder) {
	if n.matcher != "" {
		w.WriteString(n.matcher)
		w.WriteString(" {")
	}
	for i, d := range n.directives {
		if d.name == "" {
			continue // mixin
		}
		if i > 0 || n.matcher != "" {
			w.WriteString(" ")
		}
		w.WriteString(d.name)
		w.WriteString(": ")
		w.WriteString(d.value)
		w.WriteString(";")
	}
	for _, child := range n.children {
		w.WriteString(" ")
		child.print(w)
	}
	if n.matcher != "" {
		w.WriteString(" }")
	}
}

type parser struct {
	read func(name string) ([]byte, error)
	root *node
}

func (p *parser) parse(f string) error {
	p.root = &node{
		f:      f,
		vars:   []map[string]string{{}},
		mixins: make(map[string][]node),
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

func (p *parser) eval() {
	p.root.evalMatcher()
	p.root.evalDirectives()
}

func (p *parser) print() string {
	w := strings.Builder{}
	p.root.print(&w)
	return strings.TrimLeft(w.String(), " ")
}
