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
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Words struct {
	Encode      string `json:"encode"`
	TextWords   int64  `json:"textWords"`
	HeadWords   int64  `json:"headWords"`
	Outside     int64  `json:"outside"`
	Headers     int64  `json:"headers"`
	Elements    int64  `json:"elements"`
	MathInline  int64  `json:"mathInline"`
	MathDisplay int64  `json:"mathDisplay"`
	Errors      int64  `json:"errors"`
	Messages    string `json:"messages"`
}
type WordCountRequest struct {
	CommonRequestOptions
	FileName sharedTypes.PathName `json:"fileName"`
}

func (r *WordCountRequest) Preprocess() error {
	if r.FileName == "" {
		r.FileName = "main.tex"
	}
	return nil
}

func (r *WordCountRequest) Validate() error {
	if err := r.CommonRequestOptions.Validate(); err != nil {
		return err
	}
	if err := r.FileName.Validate(); err != nil {
		return err
	}
	return nil
}

type WordCountResponse struct {
	TexCount Words `json:"texcount"`
}
