// Golang port of Overleaf
// Copyright (C) 2022 Jakob Ackermann <das7pad@outlook.com>
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

func minifyTemplate(raw *template.Template, funcMap template.FuncMap) (*template.Template, error) {
	s := flatten(raw)

	// `{{- "" -}}` gets collapsed into `{{""}}` when printing a template.
	s = regexp.MustCompile(`{{""}}`).ReplaceAllString(s, "")

	// Preserve sequence of blocks in tables and similar conditions.
	blocks := regexp.MustCompile(`{{[\s\S]*?}}`)
	s = blocks.ReplaceAllStringFunc(s, func(m string) string {
		return "<!--" + html.EscapeString(m) + "-->"
	})

	s, errSwap := swapScriptNodeType(s)
	if errSwap != nil {
		return nil, errSwap
	}

	h, errParse := html.Parse(bytes.NewBufferString(s))
	if errParse != nil {
		return nil, errors.Tag(errParse, "html.Parse")
	}

	trimTextNodes(h)

	b := &bytes.Buffer{}
	b.Grow(len(s))
	if errRender := html.Render(b, h); errRender != nil {
		return nil, errors.Tag(errRender, "html.Render")
	}
	s = b.String()

	s, errSwap = swapScriptNodeType(s)
	if errSwap != nil {
		return nil, errSwap
	}

	blocksRev := regexp.MustCompile(`(<|&lt;)!--{{[\s\S]*?}}--(>|&gt;)`)
	s = blocksRev.ReplaceAllStringFunc(s, func(m string) string {
		m = html.UnescapeString(m)
		return m[4 : len(m)-3]
	})

	// `{{if foo}}\n  {{bar}}\n  {{end}}` -> `{{if foo}}{{bar}}{{end}}`
	siblingActions := regexp.MustCompile(`}}\s+{{`)
	s = siblingActions.ReplaceAllString(s, "}}{{")

	return template.New(raw.Name()).Funcs(funcMap).Parse(s)
}

func swapScriptNodeType(s string) (string, error) {
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
	b := &strings.Builder{}
	b.Grow(len(s))
	if err := html.Render(b, h); err != nil {
		return "", errors.Tag(err, "html.Render in swapScriptNodeType")
	}
	return b.String(), nil
}

func trimTextNodes(node *html.Node) {
	for next := node.FirstChild; next != nil; {
		c := next
		next = next.NextSibling

		if c.Type != html.TextNode {
			trimTextNodes(c)
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

		if start > 0 || end < n {
			c.Data = c.Data[start:end]
		}
		if node.Data != "textarea" && node.Data != "code" {
			c.Data = strings.ReplaceAll(c.Data, "\n", " ")
			c.Data = strings.ReplaceAll(c.Data, "  ", " ")
		}
	}
}

func flatten(raw *template.Template) string {
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
