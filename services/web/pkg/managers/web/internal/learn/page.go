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
	"context"
	"encoding/json"
	"net/url"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/templates"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) fetchPage(ctx context.Context, path string) (*pageContent, error) {
	u := m.apiURL.WithQuery(url.Values{
		"action":    {"parse"},
		"format":    {"json"},
		"redirects": {"true"},
		"page":      {path},
	})

	body, cleanup, err := m.proxy.Fetch(ctx, u)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	raw := pageContentRaw{}
	if err = json.NewDecoder(body).Decode(&raw); err != nil {
		return nil, errors.Tag(err, "parse api response")
	}
	return raw.parse(m.ps), nil
}

func (m *manager) getPage(ctx context.Context, path string) (*pageContent, bool, error) {
	m.pageMux.RLock()
	pc, exists := m.pageCache[path]
	m.pageMux.RUnlock()
	if exists && time.Since(pc.fetchedAt) < m.cacheDuration {
		return pc, true, nil
	}
	freshPc, err := m.fetchPage(ctx, path)
	if err != nil {
		if exists {
			// fallback to cache
			return pc, true, nil
		}
		return nil, false, err
	}

	freshPc.fetchedAt = time.Now()
	m.pageMux.Lock()
	m.pageCache[path] = freshPc
	m.pageMux.Unlock()
	pc = freshPc
	return pc, false, nil
}

func (m *manager) LearnPageEarlyRedirect(ctx context.Context, r *types.LearnPageRequest, u *url.URL) (string, int64) {
	r.Preprocess()
	if err := r.Validate(); err != nil {
		return "", 0
	}

	// NOTE: We cannot handle the error yet. The error page needs details from
	//        the session. Defer user facing error handling until .LearnPage().
	page, hit, err := m.getPage(ctx, r.WikiPage())
	if err == nil && page.redirect != "" {
		return page.redirect, page.Age(hit)
	}

	if expected := r.EscapedPath(); expected != u.EscapedPath() {
		return m.ps.SiteURL.WithPath(r.Path()).String(), -1
	}
	return "", 0
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
			return errors.Tag(err, "get contents")
		}
		cc = content
		return nil
	})
	eg.Go(func() error {
		content, hit, err := m.getPage(pCtx, r.WikiPage())
		if err != nil {
			return errors.Tag(err, "get page")
		}
		pc = content
		response.Age = pc.Age(hit)
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
				Session:     r.Session.PublicData,
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
