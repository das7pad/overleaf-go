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

package resourceCleanup

import (
	"regexp"
	"strings"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/rootDocAlias"
)

var PreserveFilesPrefixes = []string{
	// knitr cache
	"cache/",
	"output.",
	// Tikz cache figures
	"output-",
}

//goland:noinspection SpellCheckingInspection
var PreserveFilesSuffixes = []string{
	// knitr cache
	".aux",
	// Tikz cached figures
	".dpth",
	".md5",
	".pdf",
	// minted
	".pygstyle",
	".pygtex",
	// markdown
	".md.tex",
	// Epstopdf generated files
	"-eps-converted-to.pdf",
}

var PreserveRegex = regexp.MustCompile(
	// minted cache
	// markdown cache
	"^(.+/)?(_minted-|_markdown_)[^/]+/.+$",
)

func ShouldDelete(file sharedTypes.PathName) bool {
	isGenericOutputFile := file == "output.pdf" ||
		file == "output.dvi" ||
		file == "output.log" ||
		file == "output.xdv"
	if isGenericOutputFile {
		return true
	}
	if file == rootDocAlias.AliasDocFileName {
		return true
	}

	s := string(file)
	for _, prefix := range PreserveFilesPrefixes {
		if strings.HasPrefix(s, prefix) {
			return false
		}
	}
	for _, suffix := range PreserveFilesSuffixes {
		if strings.HasSuffix(s, suffix) {
			return false
		}
	}
	if PreserveRegex.MatchString(s) {
		return false
	}

	return true
}
