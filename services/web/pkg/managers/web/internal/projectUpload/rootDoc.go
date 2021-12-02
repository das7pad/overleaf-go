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

package projectUpload

import (
	"regexp"

	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

const limitSearchDocumentClass = 30 * 1024

var (
	regexHasDocumentClass = regexp.MustCompile("(^|\n)\\s*\\\\documentclass")
	regexTitleCurly       = regexp.MustCompile("\\\\[tT]itle\\*?\\s*{([^}]+)}")
	regexTitleSquare      = regexp.MustCompile("\\\\[tT]itle\\s*\\[([^]]+)]")
)

func scanContent(snapshot sharedTypes.Snapshot) (bool, project.Name) {
	var s string
	if len(snapshot) > limitSearchDocumentClass {
		s = string(snapshot[:limitSearchDocumentClass])
	} else {
		s = string(snapshot)
	}
	if !regexHasDocumentClass.MatchString(s) {
		return false, ""
	}
	if curly := regexTitleCurly.FindStringSubmatch(s); len(curly) > 0 {
		return true, project.Name(curly[1])
	}
	if square := regexTitleSquare.FindStringSubmatch(s); len(square) > 0 {
		return true, project.Name(square[1])
	}
	return true, ""
}
