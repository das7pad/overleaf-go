// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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

package listenAddress

import (
	"fmt"
	"strings"

	"github.com/das7pad/overleaf-go/pkg/options/env"
)

func Parse(port int) []string {
	return parse(
		env.GetString("LISTEN_ADDRESS", "localhost"),
		env.GetInt("PORT", port),
	)
}

func ParseOverride(l, p string, port int) []string {
	return parse(env.GetString(l, "localhost"), env.GetInt(p, port))
}

func parse(raw string, port int) []string {
	port = env.GetInt("PORT", port)
	o := strings.Split(raw, ",")
	for i, addr := range o {
		if strings.HasPrefix(addr, "/") {
			continue
		}
		if strings.ContainsRune(addr, ':') {
			continue
		}
		o[i] = fmt.Sprintf("%s:%d", addr, port)
	}
	return o
}
