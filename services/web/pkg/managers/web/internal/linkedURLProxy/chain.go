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

package linkedURLProxy

import (
	"net/url"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func chainURL(u *sharedTypes.URL, chain []sharedTypes.URL) string {
	for _, next := range chain {
		v := next.WithQuery(url.Values{"url": {u.String()}})
		u = &v
	}
	return u.String()
}
