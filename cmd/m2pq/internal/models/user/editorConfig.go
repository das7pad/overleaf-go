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

package user

import (
	"github.com/das7pad/overleaf-go/pkg/models/user"
	spellingTypes "github.com/das7pad/overleaf-go/services/spelling/pkg/types"
)

type EditorConfig struct {
	FontFamily         string                           `bson:"fontFamily"`
	FontSize           int64                            `bson:"fontSize"`
	LineHeight         string                           `bson:"lineHeight"`
	Mode               string                           `bson:"mode"`
	OverallTheme       string                           `bson:"overallTheme"`
	PDFViewer          string                           `bson:"pdfViewer"`
	SpellCheckLanguage spellingTypes.SpellCheckLanguage `bson:"spellCheckLanguage"`
	Theme              string                           `bson:"theme"`
	AutoComplete       bool                             `bson:"autoComplete"`
	AutoPairDelimiters bool                             `bson:"autoPairDelimiters"`
	SyntaxValidation   bool                             `bson:"syntaxValidation"`
}

func (e EditorConfig) Migrate() user.EditorConfig {
	return user.EditorConfig{
		AutoComplete:       e.AutoComplete,
		AutoPairDelimiters: e.AutoPairDelimiters,
		FontFamily:         e.FontFamily,
		FontSize:           e.FontSize,
		LineHeight:         e.LineHeight,
		Mode:               e.Mode,
		OverallTheme:       e.OverallTheme,
		PDFViewer:          e.PDFViewer,
		SpellCheckLanguage: e.SpellCheckLanguage,
		SyntaxValidation:   e.SyntaxValidation,
		Theme:              e.Theme,
	}
}
