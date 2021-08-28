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
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/constants"
)

type Page int64

func (p Page) String() string {
	return sharedTypes.Int(p).String()
}

type Horizontal float64

func (h Horizontal) String() string {
	return sharedTypes.Float(h).String()
}

type Vertical float64

func (v Vertical) String() string {
	return sharedTypes.Float(v).String()
}

type Row int64

func (r Row) String() string {
	return sharedTypes.Int(r).String()
}

type Column int64

func (c Column) String() string {
	return sharedTypes.Int(c).String()
}

type CodePosition struct {
	FileName FileName `json:"file"`
	Row      Row      `json:"line"`
	Column   Column   `json:"column"`
}

type CodePositions []*CodePosition

type Height float64
type Width float64

type PDFPosition struct {
	Page       Page       `json:"page"`
	Horizontal Horizontal `json:"h"`
	Vertical   Vertical   `json:"v"`
	Height     Height     `json:"height"`
	Width      Width      `json:"width"`
}
type PDFPositions []*PDFPosition

type SyncTexRequestCommon interface {
	Options() *SyncTexOptions
	CommandLine() CommandLine
}

type SyncTexOptions struct {
	BuildId      BuildId      `json:"build_id"`
	CompileGroup CompileGroup `json:"compile_group"`
	ImageName    ImageName    `json:"image_name"`
}

func (o *SyncTexOptions) Preprocess(options *Options) error {
	if o.BuildId == "" {
		o.BuildId = allZeroBuildId
	}
	if o.CompileGroup == "" {
		o.CompileGroup = options.DefaultCompileGroup
	}
	if o.ImageName == "" {
		o.ImageName = options.DefaultImage
	}
	return nil
}

func (o *SyncTexOptions) Validate(options *Options) error {
	if err := o.BuildId.Validate(options); err != nil {
		return err
	}
	if err := o.CompileGroup.Validate(options); err != nil {
		return err
	}
	if err := o.ImageName.Validate(options); err != nil {
		return err
	}
	return nil
}

func (o SyncTexOptions) getPathFor(name FileName) string {
	if o.BuildId == allZeroBuildId {
		return CompileDir(constants.CompileDirPlaceHolder).
			Join(name)
	}
	return OutputDir(constants.OutputDirPlaceHolder).
		CompileOutputDir(o.BuildId).
		Join(name)
}

func (o SyncTexOptions) OutputPDFPath() string {
	return o.getPathFor("output.pdf")
}

func (o SyncTexOptions) OutputSyncTexGzPath() string {
	return o.getPathFor("output.synctex.gz")
}

type SyncFromCodeRequest struct {
	*SyncTexOptions
	FileName FileName `json:"file_name"`
	Row      Row      `json:"line"`
	Column   Column   `json:"column"`
}

func (r *SyncFromCodeRequest) Options() *SyncTexOptions {
	return r.SyncTexOptions
}

func (r *SyncFromCodeRequest) CommandLine() CommandLine {
	line := r.Row.String()
	column := r.Column.String()
	input := string(constants.CompileDirPlaceHolder + "/" + r.FileName)
	return CommandLine{
		"synctex",
		"view",
		"-i",
		line + ":" + column + ":" + input,
		"-o",
		r.SyncTexOptions.OutputPDFPath(),
	}
}

func (r *SyncFromCodeRequest) Validate(options *Options) error {
	if err := r.FileName.Validate(); err != nil {
		return err
	}
	if r.SyncTexOptions == nil {
		return &errors.ValidationError{Msg: "missing SyncTexOptions"}
	}
	if err := r.SyncTexOptions.Validate(options); err != nil {
		return err
	}
	return nil
}

type SyncFromPDFRequest struct {
	*SyncTexOptions
	Page       Page       `json:"page"`
	Horizontal Horizontal `json:"horizontal"`
	Vertical   Vertical   `json:"vertical"`
}

func (r *SyncFromPDFRequest) Options() *SyncTexOptions {
	return r.SyncTexOptions
}

func (r *SyncFromPDFRequest) CommandLine() CommandLine {
	page := r.Page.String()
	x := r.Horizontal.String()
	y := r.Vertical.String()
	return CommandLine{
		"synctex",
		"edit",
		"-o",
		page + ":" + x + ":" + y + ":" + r.SyncTexOptions.OutputPDFPath(),
	}
}

func (r *SyncFromPDFRequest) Validate(options *Options) error {
	if r.SyncTexOptions == nil {
		return &errors.ValidationError{Msg: "missing SyncTexOptions"}
	}
	if err := r.SyncTexOptions.Validate(options); err != nil {
		return err
	}
	return nil
}
