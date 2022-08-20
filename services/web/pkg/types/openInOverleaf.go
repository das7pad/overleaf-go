// Golang port of Overleaf
// Copyright (C) 2021-2022 Jakob Ackermann <das7pad@outlook.com>
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
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/pkg/templates"
)

type OpenInOverleafSnippet struct {
	File     CreateProjectFileWithCleanup `json:"-"`
	Path     sharedTypes.PathName         `json:"path"`
	Snapshot sharedTypes.Snapshot         `json:"snapshot"`
	URL      *sharedTypes.URL             `json:"url"`
}

func (s *OpenInOverleafSnippet) Validate() error {
	if err := s.Path.Validate(); err != nil {
		return err
	}
	if s.URL != nil {
		if err := s.URL.Validate(); err != nil {
			return err
		}
		if len(s.Snapshot) > 0 {
			return &errors.ValidationError{
				Msg: "cannot specify snapshot and url",
			}
		}
	} else {
		if err := s.Snapshot.Validate(); err != nil {
			return err
		}
	}
	return nil
}

type OpenInOverleafRequest struct {
	WithSession

	Compiler       sharedTypes.Compiler    `json:"compiler"`
	HasDefaultName bool                    `json:"-"`
	ProjectName    project.Name            `json:"project_name"`
	Snippets       []OpenInOverleafSnippet `json:"snippets"`
	ZipURL         *sharedTypes.URL        `json:"zip_url"`
}

func (r *OpenInOverleafRequest) Preprocess() {
	if r.ProjectName == "" {
		r.HasDefaultName = true
		r.ProjectName = "Untitled"
	}
	if r.Compiler == "" {
		r.Compiler = project.DefaultCompiler
	}

	hasMainTex := false
	nInlinedDocs := 0
	for _, snippet := range r.Snippets {
		if snippet.URL == nil {
			nInlinedDocs++
		} else if snippet.Path == "" {
			snippet.Path = sharedTypes.PathName(snippet.URL.FileNameFromPath())
		}
		if snippet.Path == "main.tex" {
			hasMainTex = true
		}
	}
	untitledDocNum := 0
	for _, snippet := range r.Snippets {
		if snippet.URL != nil {
			continue
		}
		if snippet.Path == "" && !hasMainTex && untitledDocNum == 0 {
			snippet.Path = "main.tex"
		}
		if snippet.Path.Type() == "" {
			switch {
			case nInlinedDocs == 1 && !hasMainTex:
				snippet.Path = "main.tex"
			case snippet.Path != "":
				snippet.Path += ".tex"
			default:
				untitledDocNum++
				snippet.Path = sharedTypes.PathName(
					fmt.Sprintf("untitled-doc-%d.tex", untitledDocNum),
				)
			}
		}
	}
}

func (r *OpenInOverleafRequest) Validate() error {
	if r.Compiler != "" {
		if err := r.Compiler.Validate(); err != nil {
			return &errors.ValidationError{
				Msg: fmt.Sprintf("compiler: %s", err.Error()),
			}
		}
	}
	if err := r.ProjectName.Validate(); err != nil {
		return &errors.ValidationError{
			Msg: fmt.Sprintf("project_name: %s", err.Error()),
		}
	}
	if len(r.Snippets) > 0 && r.ZipURL != nil {
		return &errors.ValidationError{
			Msg: "specify one of snippets or zip url",
		}
	}
	for i, snippet := range r.Snippets {
		if err := snippet.Validate(); err != nil {
			return &errors.ValidationError{
				Msg: fmt.Sprintf("snippets[%d]: %s", i, err.Error()),
			}
		}
	}
	if r.ZipURL != nil {
		if err := r.ZipURL.Validate(); err != nil {
			return &errors.ValidationError{
				Msg: fmt.Sprintf("zip_url: %s", err.Error()),
			}
		}
	}
	return nil
}

func (r *OpenInOverleafRequest) checkSpecifiedExactlyOneSource(params url.Values) error {
	n := 0
	for _, s := range [4]string{"zip_uri", "snip", "snip_uri", "encoded_snip"} {
		if params.Has(s) {
			n++
		}
	}
	if n != 1 {
		return &errors.ValidationError{
			Msg: "specify one of snip/snip_uri/encoded_snip/zip_uri",
		}
	}
	return nil
}

func (r *OpenInOverleafRequest) PopulateFromParams(params url.Values) error {
	if err := r.checkSpecifiedExactlyOneSource(params); err != nil {
		return err
	}
	if s := params.Get("engine"); s != "" {
		c := sharedTypes.Compiler(s)
		//goland:noinspection SpellCheckingInspection
		if c == "latex_dvipdf" {
			c = sharedTypes.LaTeX
		}
		if err := c.Validate(); err != nil {
			return errors.Tag(err, "engine")
		}
		r.Compiler = c
	}
	if s := params.Get("zip_uri"); s != "" {
		u, err := sharedTypes.ParseAndValidateURL(s)
		if err != nil {
			return errors.Tag(err, "zip_uri")
		}
		r.ZipURL = u
		// Zip based request
		return nil
	}
	// Snippet based request
	snippets := make(
		[]OpenInOverleafSnippet,
		0,
		len(params["snip"])+
			len(params["snip_uri"])+
			len(params["encoded_snip"]),
	)
	for i, raw := range params["encoded_snip"] {
		v, err := url.QueryUnescape(raw)
		if err != nil {
			return &errors.ValidationError{
				Msg: fmt.Sprintf("encoded_snip[%d]: %s", i, err.Error()),
			}
		}

		snippets = append(snippets, OpenInOverleafSnippet{
			Snapshot: sharedTypes.Snapshot(v),
		})
	}
	for _, raw := range params["snip"] {
		snippets = append(snippets, OpenInOverleafSnippet{
			Snapshot: sharedTypes.Snapshot(raw),
		})
	}
	for i, raw := range params["snip_uri"] {
		u, err := sharedTypes.ParseAndValidateURL(raw)
		if err != nil {
			return &errors.ValidationError{
				Msg: fmt.Sprintf("snip_uri[%d]: %s", i, err.Error()),
			}
		}
		snippets = append(snippets, OpenInOverleafSnippet{
			URL: u,
		})
	}
	for i, snippet := range snippets {
		if len(params["snip_name"]) > i {
			path := sharedTypes.PathName(params["snip_name"][i])
			if err := path.Validate(); err != nil {
				return &errors.ValidationError{
					Msg: fmt.Sprintf("snip_name[%d]: %s", i, err.Error()),
				}
			}
			snippet.Path = path
		}
	}
	r.Snippets = snippets
	if r.ProjectName == "" {
		if len(params["snip_name"]) == 1 {
			name := project.Name(params["snip_name"][0])
			if err := name.Validate(); err == nil {
				r.HasDefaultName = true
				r.ProjectName = name
			}
		}
	}
	return nil
}

type OpenInOverleafDocumentationPageRequest struct {
	WithSession
}

type OpenInOverleafDocumentationPageResponse struct {
	Data *templates.OpenInOverleafDocumentationData
}

type OpenInOverleafGatewayPageRequest struct {
	WithSession
	Query url.Values      `form:"-"`
	Body  json.RawMessage `form:"-"`
}

type OpenInOverleafGatewayPageResponse struct {
	Data *templates.OpenInOverleafGatewayData
}
