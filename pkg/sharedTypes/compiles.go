// Golang port of Overleaf
// Copyright (C) 2022-2024 Jakob Ackermann <das7pad@outlook.com>
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

package sharedTypes

import (
	"regexp"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

const (
	LaTeX    = Compiler("latex")
	LuaLaTeX = Compiler("lualatex")
	PDFLaTeX = Compiler("pdflatex")
	XeLaTeX  = Compiler("xelatex")
)

type Compiler string

func (c Compiler) Validate() error {
	switch c {
	case LaTeX, LuaLaTeX, PDFLaTeX, XeLaTeX:
		return nil
	default:
		return &errors.ValidationError{Msg: "compiler not allowed"}
	}
}

func (c Compiler) LaTeXmkFlag() string {
	switch c {
	case LaTeX:
		//goland:noinspection SpellCheckingInspection
		return "-pdfdvi"
	case LuaLaTeX:
		return "-lualatex"
	case PDFLaTeX:
		return "-pdf"
	case XeLaTeX:
		return "-xelatex"
	default:
		return ""
	}
}

const (
	StandardCompileGroup = CompileGroup("standard")
	PriorityCompileGroup = CompileGroup("priority")
)

var validCompileGroups = []CompileGroup{
	StandardCompileGroup,
	PriorityCompileGroup,
}

type CompileGroup string

func (c CompileGroup) Validate() error {
	if c == "" {
		return &errors.ValidationError{Msg: "compileGroup missing"}
	}
	for _, compileGroup := range validCompileGroups {
		if c == compileGroup {
			return nil
		}
	}
	return &errors.ValidationError{Msg: "compileGroup is not allowed"}
}

type ComputeTimeout time.Duration

const MaxComputeTimeout = ComputeTimeout(10 * time.Minute)

var errTimeoutTooHigh = &errors.ValidationError{
	Msg: "timeout must be below " + time.Duration(MaxComputeTimeout).String(),
}

func (t ComputeTimeout) Validate() error {
	if t <= 0 {
		return &errors.ValidationError{Msg: "timeout must be greater zero"}
	}
	if t > MaxComputeTimeout {
		return errTimeoutTooHigh
	}
	return nil
}

func (t ComputeTimeout) String() string {
	return time.Duration(t).String()
}

var imageNameYearRegex = regexp.MustCompile(`:.*(20\d\d)\b`)

type ImageYear string

type ImageName string

func (i ImageName) Year() ImageYear {
	m := imageNameYearRegex.FindStringSubmatch(string(i))
	return ImageYear(m[1])
}

func (i ImageName) CheckIsAllowed(allowedImages []ImageName) error {
	for _, image := range allowedImages {
		if i == image {
			return nil
		}
	}
	return &errors.ValidationError{Msg: "imageName is not allowed"}
}

func (i ImageName) Validate() error {
	if i == "" {
		return &errors.ValidationError{
			Msg: "imageName missing",
		}
	}
	if !imageNameYearRegex.MatchString(string(i)) {
		return &errors.ValidationError{
			Msg: "imageName must have year in the tag, e.g. texlive:TL2022.4",
		}
	}
	return nil
}

type ProjectOptions struct {
	CompileGroup CompileGroup   `json:"c"`
	ProjectId    UUID           `json:"p"`
	UserId       UUID           `json:"u"`
	Timeout      ComputeTimeout `json:"t"`
}
