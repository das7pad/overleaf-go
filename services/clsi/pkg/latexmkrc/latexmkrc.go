// Golang port of Overleaf
// Copyright (C) 2024 Jakob Ackermann <das7pad@outlook.com>
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

package latexmkrc

import (
	"embed"
	"strings"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/constants"
)

//go:embed latexmkrc.tmpl
var files embed.FS

func Build() (string, error) {
	blob, err := files.ReadFile("latexmkrc.tmpl")
	if err != nil {
		return "", err
	}
	s := string(blob)
	replacements := map[string]string{
		"PDF_CACHING_XREF_FILENAME": constants.PDFCachingXrefFilename,
	}
	for needle, replacement := range replacements {
		if !strings.Contains(s, needle) {
			return "", errors.New(needle + " not found in latexmkrc template")
		}
		s = strings.ReplaceAll(s, needle, replacement)
	}

	files = embed.FS{}
	return s, nil
}
