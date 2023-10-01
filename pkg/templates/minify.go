// Golang port of Overleaf
// Copyright (C) 2022-2023 Jakob Ackermann <das7pad@outlook.com>
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

package templates

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

func newMinifier() minifier {
	return minifier{
		blocks:         regexp.MustCompile(`{{[\s\S]*?}}`),
		siblingActions: regexp.MustCompile(`}}\s+{{`),
	}
}

type minifier struct {
	siblingActions *regexp.Regexp
	blocks         *regexp.Regexp
}

func (m minifier) MinifyTemplate(raw *template.Template, funcMap template.FuncMap) (*template.Template, error) {
	var err error
	s := m.flatten(raw)

	// `{{- "" -}}` gets collapsed into `{{""}}` when printing a template.
	s = strings.ReplaceAll(s, `{{""}}`, "")

	// Preserve sequence of blocks in tables and similar conditions.
	escaped := make(map[string]string)
	s = m.blocks.ReplaceAllStringFunc(s, func(m string) string {
		e := html.EscapeString(m)
		ec := html.Token{
			Type: html.CommentToken,
			Data: m,
		}.String()
		ec = ec[4 : len(ec)-3]
		escaped["<!--"+e+"-->"] = m
		escaped["<!--"+ec+"-->"] = m
		escaped["&lt;!--"+e+"--&gt;"] = m
		escaped["&lt;!--"+ec+"--&gt;"] = m
		return "<!--" + e + "-->"
	})

	s, err = m.swapScriptNodeType(s)
	if err != nil {
		return nil, err
	}

	h, err := html.Parse(bytes.NewBufferString(s))
	if err != nil {
		return nil, errors.Tag(err, "html.Parse")
	}

	m.trimTextNodes(h)

	b := strings.Builder{}
	b.Grow(len(s))
	if err = html.Render(&b, h); err != nil {
		return nil, errors.Tag(err, "html.Render")
	}
	s = b.String()

	s, err = m.swapScriptNodeType(s)
	if err != nil {
		return nil, err
	}

	// `<!-- ESCAPED_BLOCK -->` -> BLOCK
	for needle, original := range escaped {
		s = strings.ReplaceAll(s, needle, original)
	}

	// `{{if foo}}\n  {{bar}}\n  {{end}}` -> `{{if foo}}{{bar}}{{end}}`
	s = m.siblingActions.ReplaceAllString(s, "}}{{")

	return template.New(raw.Name()).Funcs(funcMap).Parse(s)
}

func (m minifier) swapScriptNodeType(s string) (string, error) {
	h, errParse := html.Parse(bytes.NewBufferString(s))
	if errParse != nil {
		return "", errors.Tag(errParse, "html.Parse in swapScriptNodeType")
	}
	var process func(node *html.Node) error
	process = func(node *html.Node) error {
		for _, attribute := range node.Attr {
			if attribute.Key != "type" {
				continue
			}
			if attribute.Val != "text/ng-template" {
				break
			}
			if node.Data == "script" {
				node.Data = "div"
				node.DataAtom = atom.Div
				hn, err := html.Parse(
					bytes.NewBufferString(node.FirstChild.Data),
				)
				if err != nil {
					return err
				}
				// tree -> html -> head -> body -> actual node
				node.FirstChild = hn.FirstChild.FirstChild.NextSibling.FirstChild
			} else {
				node.Data = "script"
				node.DataAtom = atom.Script
			}
			return nil
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.ElementNode {
				continue
			}
			if err := process(c); err != nil {
				return err
			}
		}
		return nil
	}
	if err := process(h); err != nil {
		return "", err
	}
	b := strings.Builder{}
	b.Grow(len(s))
	if err := html.Render(&b, h); err != nil {
		return "", errors.Tag(err, "html.Render in swapScriptNodeType")
	}
	return b.String(), nil
}

func (m minifier) trimTextNodes(node *html.Node) {
	for next := node.FirstChild; next != nil; {
		c := next
		next = next.NextSibling

		if c.Type != html.TextNode {
			m.trimTextNodes(c)
			continue
		}
		start := 0
		n := len(c.Data)
		for ; start < n; start++ {
			if c.Data[start] == ' ' || c.Data[start] == '\n' {
				continue
			}
			break
		}
		if start == n {
			node.RemoveChild(c)
			continue
		}

		end := n
		for ; end > 0; end-- {
			if c.Data[end-1] == ' ' || c.Data[end-1] == '\n' {
				continue
			}
			break
		}

		// retain one leading/trailing space
		if start > 0 {
			start--
		}
		if end < n {
			end++
		}

		if node.Data != "textarea" && node.Data != "code" {
			if start > 0 || end < n {
				c.Data = c.Data[start:end]
			}
			c.Data = strings.ReplaceAll(c.Data, "\n", " ")
			c.Data = strings.ReplaceAll(c.Data, "  ", " ")
		}
	}
}

func (m minifier) flatten(raw *template.Template) string {
	s := ""
	for _, t := range raw.Templates() {
		if t.Name() == raw.Name() {
			s = t.Tree.Root.String()
		}
	}
	for i := 0; i < len(raw.Templates()); i++ {
		for _, t := range raw.Templates() {
			if t.Name() == "" {
				continue
			}
			partial := t.Tree.Root.String()
			s = strings.ReplaceAll(
				s,
				fmt.Sprintf("{{template %q .}}", t.Name()),
				partial,
			)
		}
	}
	return s
}
