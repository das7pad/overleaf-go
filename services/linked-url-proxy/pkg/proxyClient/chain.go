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

package proxyClient

import (
	"net/url"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/linked-url-proxy/pkg/constants"
)

func chainURL(u *sharedTypes.URL, chain []sharedTypes.URL) string {
	for i, next := range chain {
		q := url.Values{constants.QueryNameURL: {u.String()}}
		if i > 0 {
			q.Set(constants.QueryNameProxyChainMarker, "true")
		}
		u = next.WithQuery(q)
	}
	return u.String()
}
