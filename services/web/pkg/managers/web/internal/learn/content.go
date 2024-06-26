// Golang port of Overleaf
// Copyright (C) 2021-2023 Jakob Ackermann <das7pad@outlook.com>
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

package learn

import (
	"html/template"
	"regexp"
	"strings"
	"time"

	"github.com/das7pad/overleaf-go/pkg/templates"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

//goland:noinspection SpellCheckingInspection
type pageContentRaw struct {
	Parse struct {
		Categories []struct {
			Star string `json:"*"`
		} `json:"categories"`
		Text struct {
			Star string `json:"*"`
		} `json:"text"`
		Title     string `json:"title"`
		RevId     int64  `json:"revid"`
		Redirects []struct {
			To string `json:"to"`
		} `json:"redirects"`
	} `json:"parse"`
}

func (pc *pageContentRaw) isHidden() bool {
	for _, category := range pc.Parse.Categories {
		if category.Star == "Hide" {
			return true
		}
	}
	return false
}

func (pc *pageContentRaw) redirect() string {
	to := ""
	// Pick the last non-empty redirect.
	for _, redirect := range pc.Parse.Redirects {
		if redirect.To != "" {
			to = redirect.To
		}
	}
	if to == "" {
		return ""
	}
	r := types.LearnPageRequest{}
	{
		s := strings.TrimPrefix(to, "/")
		r.Section, s, _ = strings.Cut(s, "/")
		r.SubSection, s, _ = strings.Cut(s, "/")
		r.Page, s, _ = strings.Cut(s, "/")
		if s != "" {
			// Unexpected URL schema, /section/subsection/page/...
			return to
		}
	}
	if r.Page == "" {
		r.Page = r.SubSection
		r.SubSection = ""
	}
	r.Preprocess()
	if err := r.Validate(); err != nil {
		return to
	}
	return strings.TrimPrefix(r.Path(), "/learn")
}

var regexOverleafLinks = regexp.MustCompile(
	"https://www.overleaf.com(/docs|/learn)",
)

func (pc *pageContentRaw) parse(ps *templates.PublicSettings) *pageContent {
	if pc.Parse.RevId == 0 {
		// Page not found.
		return &pageContent{exists: false}
	}
	if pc.isHidden() {
		return &pageContent{exists: false}
	}
	if redirect := pc.redirect(); redirect != "" {
		return &pageContent{
			redirect: ps.SiteURL.
				WithPath("/learn").
				WithPath(redirect).
				String(),
		}
	}
	html := template.HTML(regexOverleafLinks.ReplaceAllString(
		pc.Parse.Text.Star, ps.SiteURL.String()+"$1",
	))
	title := pc.Parse.Title
	if idx := strings.LastIndexByte(title, '/'); idx != -1 {
		title = title[idx+1:]
	}
	titleLocale := ""
	if title == "Main Page" {
		titleLocale = "Documentation"
		title = ""
	}
	return &pageContent{
		html:        html,
		title:       title,
		titleLocale: titleLocale,
		exists:      true,
	}
}

type pageContent struct {
	html        template.HTML
	redirect    string
	title       string
	titleLocale string
	fetchedAt   time.Time
	exists      bool
}

func (pc *pageContent) Age(hit bool) int64 {
	if hit {
		return int64(time.Since(pc.fetchedAt).Seconds())
	}
	return -1
}
