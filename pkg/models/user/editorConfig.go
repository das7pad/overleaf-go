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

package user

type EditorConfig struct {
	Mode               string `json:"mode" bson:"mode"`
	Theme              string `json:"theme" bson:"theme"`
	OverallTheme       string `json:"overallTheme" bson:"overallTheme"`
	FontSize           int    `json:"fontSize" bson:"fontSize"`
	AutoComplete       bool   `json:"autoComplete" bson:"autoComplete"`
	AutoPairDelimiters bool   `json:"autoPairDelimiters" bson:"autoPairDelimiters"`
	SpellCheckLanguage string `json:"spellCheckLanguage" bson:"spellCheckLanguage"`
	PDFViewer          string `json:"pdfViewer" bson:"pdfViewer"`
	SyntaxValidation   bool   `json:"syntaxValidation" bson:"syntaxValidation"`
	FontFamily         string `json:"fontFamily" bson:"fontFamily"`
	LineHeight         string `json:"lineHeight" bson:"lineHeight"`
}