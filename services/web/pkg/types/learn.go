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

package types

import (
	"net/url"
	"strings"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/pkg/templates"
)

type LearnPageRequest struct {
	WithSession
	Section         string `form:"-"`
	SubSection      string `form:"-"`
	Page            string `form:"-"`
	HasQuestionMark bool   `form:"-"`
}

func (r *LearnPageRequest) Preprocess() {
	if r.Section == "" && r.SubSection == "" && r.Page == "" {
		// Home
		return
	}
	switch strings.ToLower(r.Section) {
	case "latex":
		if r.SubSection == "" && r.Page == "" {
			// latex section has no overview, send Home
			r.Section = ""
		} else {
			r.Section = "latex"
		}
	case "how-to", "kb":
		r.Section = "how-to"
	default:
		if r.Page == "" {
			// /learn/foo_bar -> /learn/latex/foo_bar
			r.Page = r.Section
		} else {
			// /learn/Errors/foo -> /learn/latex/Errors/foo
			r.SubSection = r.Section
		}
		r.Section = "latex"
	}
	if r.HasQuestionMark && r.Page != "" && !strings.HasSuffix(r.Page, "?") {
		r.Page += "?"
	}
	r.Page = strings.ReplaceAll(r.Page, " ", "_")
	r.SubSection = strings.ReplaceAll(r.SubSection, " ", "_")
}

func (r *LearnPageRequest) Validate() error {
	if r.Section != "" && r.Section != "latex" && r.Section != "how-to" {
		return &errors.NotFoundError{}
	}
	for _, s := range []string{r.Section, r.SubSection, r.Page} {
		for _, needle := range []string{"help:", "special:", "template:"} {
			if strings.Contains(s, needle) {
				return &errors.UnprocessableEntityError{Msg: "internal page"}
			}
		}
	}
	return nil
}

func (r *LearnPageRequest) EscapedPath() string {
	u := "/learn"
	if r.Section != "" {
		u += "/" + url.PathEscape(r.Section)
	}
	if r.SubSection != "" {
		u += "/" + url.PathEscape(r.SubSection)
	}
	if r.Page != "" {
		u += "/" + url.PathEscape(r.Page)
	}
	return u
}

func (r *LearnPageRequest) Path() string {
	u := "/learn"
	if r.Section != "" {
		u += "/" + r.Section
	}
	if r.SubSection != "" {
		u += "/" + r.SubSection
	}
	if r.Page != "" {
		u += "/" + r.Page
	}
	return u
}

func (r *LearnPageRequest) WikiPage() string {
	switch r.Section {
	case "":
		return "Main_Page"
	case "how-to":
		if r.Page == "" {
			return "Kb/Knowledge Base"
		} else if r.SubSection != "" {
			return "Kb/" + r.SubSection + "/" + r.Page
		} else {
			return "Kb/" + r.Page
		}
	case "latex":
		if r.SubSection != "" {
			return r.SubSection + "/" + r.Page
		} else {
			return r.Page
		}
	default:
		return "417" // .Validate ensures that this code path is not reachable.
	}
}

type LearnPageResponse struct {
	Redirect string
	Age      int64
	Data     *templates.LearnPageData
}

type LearnImageRequest struct {
	Path sharedTypes.PathName `form:"-"`
}

type LearnImageResponse struct {
	FSPath string
	Age    int64
}
