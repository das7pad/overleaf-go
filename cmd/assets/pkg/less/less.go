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
	directives []directive
	children   []*node
	imports    []string
	vars       map[string]string
}

var importPrefixes = []string{"@import '", "@import (less) '"}

func (n *node) consume(read func(name string) ([]byte, error), s string) (int, error) {
nextChar:
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ', '\n':
			continue
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
			i++ // skip @
			nameEnd := strings.IndexRune(s[i:], ':')
			valueEnd := strings.IndexRune(s[i+nameEnd:], ';')
			name := strings.TrimSpace(s[i : i+nameEnd])
			value := strings.TrimSpace(s[i+nameEnd : i+valueEnd])
			if n.vars == nil {
				n.vars = make(map[string]string, 1)
			}
			n.vars[name] = value
			i += nameEnd + valueEnd
		case '}':
			return i + 1, nil
		default:
			open := strings.IndexRune(s[i:], '{')
			semi := strings.IndexRune(s[i:], ';')
			if open != -1 && len(s) > open && open < semi {
				n1 := node{
					f: n.f,
				}
				n1.matcher = strings.TrimSpace(s[i : i+open])
				n.children = append(n.children, &n1)
				off, err := n1.consume(read, s[i+open+1:])
				if err != nil {
					return i, err
				}
				i += open + off
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
	}
	return len(s), nil
}

func (n *node) print(w *strings.Builder) {
	if n.matcher != "" {
		w.WriteString(n.matcher)
		w.WriteString(" {")
		for _, d := range n.directives {
			w.WriteString(" ")
			w.WriteString(d.name)
			w.WriteString(": ")
			w.WriteString(d.value)
			w.WriteString(";")
		}
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
		f: f,
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
	p.root.print(&w)
	return strings.TrimLeft(w.String(), " ")
}
