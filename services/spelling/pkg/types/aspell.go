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

package types

import (
	"github.com/das7pad/overleaf-go/pkg/errors"
)

type Misspelling struct {
	Index       int      `json:"index"`
	Suggestions []string `json:"suggestions"`
}

var AllowedLanguages = []SpellCheckLanguage{
	"en",
	"bg",
	"de",
	"es",
	"fr",
	"pt_BR",
	"pt_PT",
}

type SpellCheckLanguage string

func (l SpellCheckLanguage) Validate() error {
	for _, language := range AllowedLanguages {
		if l == language {
			return nil
		}
	}
	return &errors.ValidationError{Msg: "non supported language specified"}
}
