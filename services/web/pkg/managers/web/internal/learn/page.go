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

package learn

import (
	"context"
	"encoding/json"
	"net/url"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/templates"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/linkedURLProxy"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) fetchPage(ctx context.Context, path string) (*pageContent, error) {
	u := m.apiURL.WithQuery(url.Values{
		"action":    {"parse"},
		"format":    {"json"},
		"redirects": {"true"},
		"page":      {path},
	})

	body, err := m.proxy.Fetch(ctx, u)
	if err != nil {
		return nil, err
	}
	defer linkedURLProxy.CleanupResponseBody(body)
	raw := &pageContentRaw{}
	if err = json.NewDecoder(body).Decode(raw); err != nil {
		return nil, errors.Tag(err, "cannot parse api response")
	}
	return raw.parse(m.ps), nil
}

func (m *manager) getPage(ctx context.Context, path string) (*pageContent, bool, error) {
	m.pageMux.RLock()
	pc, exists := m.pageCache[path]
	m.pageMux.RUnlock()
	now := time.Now()
	if exists && pc.fetchedAt.Add(m.cacheDuration).After(now) {
		return pc, true, nil
	}
	freshPc, err := m.fetchPage(ctx, path)
	if err != nil {
		return nil, false, err
	}

	freshPc.fetchedAt = time.Now()
	m.pageMux.Lock()
	m.pageCache[path] = freshPc
	m.pageMux.Unlock()
	pc = freshPc
	return pc, false, nil
}

func (m *manager) LearnPage(ctx context.Context, r *types.LearnPageRequest, response *types.LearnPageResponse) error {
	r.Preprocess()
	if err := r.Validate(); err != nil {
		return err
	}
	var pc *pageContent
	var cc *pageContent
	eg, pCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		content, _, err := m.getPage(pCtx, "Contents")
		if err != nil {
			return errors.Tag(err, "cannot get contents")
		}
		cc = content
		return nil
	})
	eg.Go(func() error {
		page := ""
		switch r.Section {
		case "":
			page = "Main_Page"
		case "how-to":
			if r.Page == "" {
				page = "Kb/Knowledge Base"
			} else {
				page = "Kb/" + r.Page
			}
		case "latex":
			page = r.Page
		}
		content, hit, err := m.getPage(pCtx, page)
		if err != nil {
			return errors.Tag(err, "cannot get contents")
		}
		pc = content
		if hit {
			response.Age = int64(time.Now().Sub(pc.fetchedAt).Seconds())
		} else {
			response.Age = -1
		}
		return nil
	})
	if err := eg.Wait(); err != nil {
		return err
	}

	if target := pc.redirect; target != "" {
		response.Redirect = target
		return nil
	}
	if !pc.exists {
		return &errors.NotFoundError{}
	}

	response.Data = &templates.LearnPageData{
		MarketingLayoutData: templates.MarketingLayoutData{
			CommonData: templates.CommonData{
				Settings:    m.ps,
				SessionUser: r.Session.User,
				Title:       pc.title,
				TitleLocale: pc.titleLocale,
				Viewport:    true,
			},
		},
		PageContent:     pc.html,
		ContentsContent: cc.html,
	}
	return nil
}
