// Golang port of the Overleaf clsi service
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
	CompileGroup CompileGroup         `json:"compileGroup"`
	FileName     sharedTypes.PathName `json:"fileName"`
	ImageName    ImageName            `json:"imageName"`
}

func (r *WordCountRequest) Preprocess() error {
	if r.FileName == "" {
		r.FileName = "main.tex"
	}
	return nil
}

func (r *WordCountRequest) Validate() error {
	if err := r.CompileGroup.Validate(); err != nil {
		return err
	}
	if err := r.FileName.Validate(); err != nil {
		return err
	}
	if err := r.ImageName.Validate(); err != nil {
		return err
	}
	return nil
}
