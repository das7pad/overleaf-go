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

import (
	"github.com/das7pad/overleaf-go/pkg/errors"
	spellingTypes "github.com/das7pad/overleaf-go/services/spelling/pkg/types"
)

type EditorConfig struct {
	AutoComplete       bool                             `json:"autoComplete" bson:"autoComplete"`
	AutoPairDelimiters bool                             `json:"autoPairDelimiters" bson:"autoPairDelimiters"`
	FontFamily         string                           `json:"fontFamily" bson:"fontFamily"`
	FontSize           int                              `json:"fontSize" bson:"fontSize"`
	LineHeight         string                           `json:"lineHeight" bson:"lineHeight"`
	Mode               string                           `json:"mode" bson:"mode"`
	OverallTheme       string                           `json:"overallTheme" bson:"overallTheme"`
	PDFViewer          string                           `json:"pdfViewer" bson:"pdfViewer"`
	SyntaxValidation   bool                             `json:"syntaxValidation" bson:"syntaxValidation"`
	SpellCheckLanguage spellingTypes.SpellCheckLanguage `json:"spellCheckLanguage" bson:"spellCheckLanguage"`
	Theme              string                           `json:"theme" bson:"theme"`
}

func (e *EditorConfig) Validate() error {
	//goland:noinspection SpellCheckingInspection
	switch e.FontFamily {
	case "monaco":
	case "lucida":
		// valid
	default:
		return &errors.ValidationError{Msg: "unknown fontFamily"}
	}

	if e.FontSize < 10 || e.FontSize > 50 {
		// The current dropdown sports 10, 11, 12, 13, 14, 16, 18, 20, 22, 24.
		return &errors.ValidationError{Msg: "invalid fontSize"}
	}

	switch e.LineHeight {
	case "compact":
	case "normal":
	case "wide":
		// valid
	default:
		return &errors.ValidationError{Msg: "unknown lineHeight"}
	}

	switch e.Mode {
	case "default":
	case "emacs":
	case "vim":
		// valid
	default:
		return &errors.ValidationError{Msg: "unknown mode"}
	}

	switch e.OverallTheme {
	case "":
	case "light-":
		// valid
	default:
		return &errors.ValidationError{Msg: "unknown overallTheme"}
	}

	switch e.PDFViewer {
	case "pdfjs":
	case "native":
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

	//goland:noinspection SpellCheckingInspection
	switch e.Theme {
	case "ambiance":
	case "chaos":
	case "chrome":
	case "clouds":
	case "clouds_midnight":
	case "cobalt":
	case "crimson_editor":
	case "dawn":
	case "dracula":
	case "dreamweaver":
	case "eclipse":
	case "github":
	case "gob":
	case "gruvbox":
	case "idle_fingers":
	case "iplastic":
	case "katzenmilch":
	case "kr_theme":
	case "kuroir":
	case "merbivore":
	case "merbivore_soft":
	case "mono_industrial":
	case "monokai":
	case "overleaf":
	case "pastel_on_dark":
	case "solarized_dark":
	case "solarized_light":
	case "sqlserver":
	case "terminal":
	case "textmate":
	case "tomorrow":
	case "tomorrow_night":
	case "tomorrow_night_blue":
	case "tomorrow_night_bright":
	case "tomorrow_night_eighties":
	case "twilight":
	case "vibrant_ink":
	case "xcode":
		// valid
	default:
		return &errors.ValidationError{Msg: "unknown theme"}
	}
	return nil
}
