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

package learn

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/pkg/templates"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/linkedURLProxy"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	LearnPage(ctx context.Context, request *types.LearnPageRequest, response *types.LearnPageResponse) error
	ProxyImage(ctx context.Context, request *types.LearnImageRequest, response *types.LearnImageResponse) error
}

func New(options *types.Options, ps *templates.PublicSettings, proxy linkedURLProxy.Manager) (Manager, error) {
	upstreamURL, err := sharedTypes.ParseAndValidateURL(
		"https://learn.overleaf.com",
	)
	if err != nil {
		return nil, err
	}

	err = os.MkdirAll(options.LearnImageCacheBase.String(), 0o755)
	if err != nil {
		return nil, errors.Tag(err, "cannot create image cache")
	}
	m := &manager{
		apiURL:        upstreamURL.WithPath("/learn-scripts/api.php"),
		baseImageURL:  upstreamURL,
		baseImagePath: options.LearnImageCacheBase,
		cacheDuration: options.LearnCacheDuration,
		proxy:         proxy,
		ps:            ps,
		pageCache:     make(map[string]*pageContent),
		imageCache:    make(map[string]time.Time),
	}
	if err = m.fillImageCache(); err != nil {
		return nil, err
	}
	if err = m.sweepImageCache(); err != nil {
		return nil, err
	}
	return m, nil
}

type manager struct {
	apiURL        *sharedTypes.URL
	baseImageURL  *sharedTypes.URL
	baseImagePath sharedTypes.DirName
	cacheDuration time.Duration
	proxy         linkedURLProxy.Manager
	ps            *templates.PublicSettings

	pageMux   sync.RWMutex
	pageCache map[string]*pageContent

	imageMux   sync.RWMutex
	imageCache map[string]time.Time
}
