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
	"fmt"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/constants"
)

type CodePosition struct {
	FileName sharedTypes.PathName `json:"file"`
	Row      int64                `json:"line"`
	Column   int64                `json:"column"`
}

type CodePositions []CodePosition

type PDFPosition struct {
	Page       int64   `json:"page"`
	Horizontal float64 `json:"h"`
	Vertical   float64 `json:"v"`
	Height     float64 `json:"height"`
	Width      float64 `json:"width"`
}
type PDFPositions []PDFPosition

type SyncTexRequestCommon interface {
	Options() *SyncTexOptions
	CommandLine() CommandLine
}

type SyncTexOptions struct {
	CommonRequestOptions
	BuildId BuildId `json:"buildId"`
}

func (o *SyncTexOptions) Validate() error {
	if err := o.CommonRequestOptions.Validate(); err != nil {
		return err
	}
	if err := o.BuildId.Validate(); err != nil {
		return err
	}
	return nil
}

func (o SyncTexOptions) getPathFor(name sharedTypes.PathName) string {
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
	SyncTexOptions
	FileName sharedTypes.PathName `json:"fileName"`
	Row      int64                `json:"line"`
	Column   int64                `json:"column"`
}

func (r *SyncFromCodeRequest) Options() *SyncTexOptions {
	return &r.SyncTexOptions
}

func (r *SyncFromCodeRequest) CommandLine() CommandLine {
	return CommandLine{
		"synctex",
		"view",
		"-i",
		fmt.Sprintf(
			"%d:%d:%s",
			r.Row, r.Column, constants.CompileDirPlaceHolder+"/"+r.FileName,
		),
		"-o",
		r.SyncTexOptions.OutputPDFPath(),
	}
}

func (r *SyncFromCodeRequest) Validate() error {
	if err := r.FileName.Validate(); err != nil {
		return err
	}
	if err := r.SyncTexOptions.Validate(); err != nil {
		return err
	}
	return nil
}

type SyncFromCodeResponse struct {
	PDF PDFPositions `json:"pdf"`
}

type SyncFromPDFRequest struct {
	SyncTexOptions
	Page       int64   `json:"page"`
	Horizontal float64 `json:"horizontal"`
	Vertical   float64 `json:"vertical"`
}

func (r *SyncFromPDFRequest) Options() *SyncTexOptions {
	return &r.SyncTexOptions
}

func (r *SyncFromPDFRequest) CommandLine() CommandLine {
	return CommandLine{
		"synctex",
		"edit",
		"-o",
		fmt.Sprintf(
			"%d:%f:%f:%s",
			r.Page, r.Horizontal, r.Vertical, r.SyncTexOptions.OutputPDFPath(),
		),
	}
}

func (r *SyncFromPDFRequest) Validate() error {
	if err := r.SyncTexOptions.Validate(); err != nil {
		return err
	}
	return nil
}

type SyncFromPDFResponse struct {
	Code CodePositions `json:"code"`
}
