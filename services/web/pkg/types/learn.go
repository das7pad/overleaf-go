// Golang port of Overleaf
// Copyright (C) 2021 Jakob Ackermann <das7pad@outlook.com>
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
	"regexp"
	"strings"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/session"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/pkg/templates"
)

type LearnPageRequest struct {
	Session         *session.Session `form:"-"`
	Section         string           `form:"-"`
	Page            string           `form:"-"`
	HasQuestionMark bool             `form:"-"`
}

func (r *LearnPageRequest) Preprocess() {
	if r.Section == "" && r.Page == "" {
		// Home
		return
	}
	switch strings.ToLower(r.Section) {
	case "latex":
		if r.Page == "" {
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
			r.Section = "latex"
		}
	}
	if r.HasQuestionMark && r.Page != "" && !strings.HasSuffix(r.Page, "?") {
		r.Page += "?"
	}
}

var learnInternalPages = regexp.MustCompile("help:|special:|template:")

func (r *LearnPageRequest) Validate() error {
	if r.Section != "" && r.Section != "latex" && r.Section != "how-to" {
		return &errors.NotFoundError{}
	}
	if learnInternalPages.MatchString(r.Page) {
		return &errors.UnprocessableEntityError{Msg: "internal page"}
	}
	return nil
}

func (r *LearnPageRequest) PreSessionRedirect(path string) string {
	r.Preprocess()
	if err := r.Validate(); err != nil {
		return ""
	}
	u := "/learn"
	if r.Section != "" {
		u += "/" + url.PathEscape(r.Section)
	}
	if r.Page != "" {
		u += "/" + url.PathEscape(r.Page)
	}
	if u != path {
		return u
	}
	return ""
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
