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

package user

import (
	"github.com/das7pad/overleaf-go/pkg/errors"
	spellingTypes "github.com/das7pad/overleaf-go/services/spelling/pkg/types"
)

type EditorConfig struct {
	AutoComplete       bool                             `json:"autoComplete" edgedb:"auto_complete"`
	AutoPairDelimiters bool                             `json:"autoPairDelimiters" edgedb:"auto_pair_delimiters"`
	FontFamily         string                           `json:"fontFamily" edgedb:"font_family"`
	FontSize           int64                            `json:"fontSize" edgedb:"font_size"`
	LineHeight         string                           `json:"lineHeight" edgedb:"line_height"`
	Mode               string                           `json:"mode" edgedb:"mode"`
	OverallTheme       string                           `json:"overallTheme" edgedb:"overall_theme"`
	PDFViewer          string                           `json:"pdfViewer" edgedb:"pdf_viewer"`
	SyntaxValidation   bool                             `json:"syntaxValidation" edgedb:"syntax_validation"`
	SpellCheckLanguage spellingTypes.SpellCheckLanguage `json:"spellCheckLanguage" edgedb:"spell_check_language"`
	Theme              string                           `json:"theme" edgedb:"theme"`
}

//goland:noinspection SpellCheckingInspection
const (
	editorFontFamilyLucida = "lucida"
	editorFontFamilyMonaco = "monaco"

	editorLineHightCompact = "compact"
	editorLineHightNormal  = "normal"
	editorLineHightWide    = "wide"

	editorModeDefault = "default"
	editorModeEmacs   = "emacs"
	editorModeVim     = "vim"

	editorOverallThemeNone  = ""
	editorOverallThemeLight = "light-"

	editorPdfViewerPdfjs  = "pdfjs"
	editorPdfViewerNative = "native"
)

var (
	//goland:noinspection SpellCheckingInspection
	EditorThemes = []string{
		"ambiance",
		"chaos",
		"chrome",
		"clouds",
		"clouds_midnight",
		"cobalt",
		"crimson_editor",
		"dawn",
		"dracula",
		"dreamweaver",
		"eclipse",
		"github",
		"gob",
		"gruvbox",
		"idle_fingers",
		"iplastic",
		"katzenmilch",
		"kr_theme",
		"kuroir",
		"merbivore",
		"merbivore_soft",
		"mono_industrial",
		"monokai",
		"overleaf",
		"pastel_on_dark",
		"solarized_dark",
		"solarized_light",
		"sqlserver",
		"terminal",
		"textmate",
		"tomorrow",
		"tomorrow_night",
		"tomorrow_night_blue",
		"tomorrow_night_bright",
		"tomorrow_night_eighties",
		"twilight",
		"vibrant_ink",
		"xcode",
	}
)

func (e *EditorConfig) Validate() error {
	switch e.FontFamily {
	case editorFontFamilyMonaco:
	case editorFontFamilyLucida:
		// valid
	default:
		return &errors.ValidationError{Msg: "unknown fontFamily"}
	}

	if e.FontSize < 10 || e.FontSize > 50 {
		// The current dropdown sports 10, 11, 12, 13, 14, 16, 18, 20, 22, 24.
		return &errors.ValidationError{Msg: "invalid fontSize"}
	}

	switch e.LineHeight {
	case editorLineHightCompact:
	case editorLineHightNormal:
	case editorLineHightWide:
		// valid
	default:
		return &errors.ValidationError{Msg: "unknown lineHeight"}
	}

	switch e.Mode {
	case editorModeDefault:
	case editorModeEmacs:
	case editorModeVim:
		// valid
	default:
		return &errors.ValidationError{Msg: "unknown mode"}
	}

	switch e.OverallTheme {
	case editorOverallThemeNone:
	case editorOverallThemeLight:
		// valid
	default:
		return &errors.ValidationError{Msg: "unknown overallTheme"}
	}

	switch e.PDFViewer {
	case editorPdfViewerPdfjs:
	case editorPdfViewerNative:
		// valid
	default:
		return &errors.ValidationError{Msg: "unknown pdfViewer"}
	}

	if e.SpellCheckLanguage == "" {
		// disable spell checking
	} else {
		if err := e.SpellCheckLanguage.Validate(); err != nil {
			return err
		}
	}

	foundTheme := false
	for _, theme := range EditorThemes {
		if theme == e.Theme {
			foundTheme = true
			break
		}
	}
	if !foundTheme {
		return &errors.ValidationError{Msg: "unknown theme"}
	}
	return nil
}
