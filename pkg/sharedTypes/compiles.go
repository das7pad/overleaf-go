// Golang port of Overleaf
// Copyright (C) 2022 Jakob Ackermann <das7pad@outlook.com>
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
	Latex    = Compiler("latex")
	LuaLatex = Compiler("lualatex")
	PDFLatex = Compiler("pdflatex")
	XeLatex  = Compiler("xelatex")
)

var validCompilers = []Compiler{
	Latex, LuaLatex, PDFLatex, XeLatex,
}

type Compiler string

func (c Compiler) Validate() error {
	for _, compiler := range validCompilers {
		if c == compiler {
			return nil
		}
	}
	return &errors.ValidationError{Msg: "compiler not allowed"}
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

var imageNameYearRegex = regexp.MustCompile(":([0-9]+)\\.[0-9]+")

type ImageName string

func (i ImageName) Year() string {
	m := imageNameYearRegex.FindStringSubmatch(string(i))
	return m[1]
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
			Msg: "imageName does not match year regex",
		}
	}
	return nil
}
