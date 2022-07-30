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
	"encoding/json"

	"github.com/jackc/pgtype"

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

func (e EditorConfig) EncodeBinary(ci *pgtype.ConnInfo, buf []byte) ([]byte, error) {
	blob, err := json.Marshal(e)
	if err != nil {
		return nil, errors.Tag(err, "serialize EditorConfig")
	}
	return pgtype.JSONB{
		Bytes:  blob,
		Status: pgtype.Present,
	}.EncodeBinary(ci, buf)
}
