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

package linkedURLProxy

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/linked-url-proxy/pkg/proxyClient"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	proxyClient.Manager
	DownloadFile(ctx context.Context, src *sharedTypes.URL) (*bufferedFile, error)
}

func New(options *types.Options) (Manager, error) {
	c, err := proxyClient.New(options.APIs.LinkedURLProxy.Chain)
	if err != nil {
		return nil, err
	}
	return &manager{
		Manager: c,
	}, nil
}

type manager struct {
	proxyClient.Manager
}
