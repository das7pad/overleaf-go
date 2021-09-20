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

package types

import (
	"strings"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

const (
	Latex    = Compiler("latex")
	LuaLatex = Compiler("lualatex")
	PDFLatex = Compiler("pdflatex")
	XeLatex  = Compiler("xelatex")
)

var ValidCompilers = []Compiler{
	Latex, LuaLatex, PDFLatex, XeLatex,
}

type Compiler string

func (c Compiler) Validate() error {
	for _, compiler := range ValidCompilers {
		if c == compiler {
			return nil
		}
	}
	return &errors.ValidationError{Msg: "compiler not allowed"}
}

type DraftModeFlag bool

func (d DraftModeFlag) Validate() error {
	return nil
}

type CheckMode string

const (
	NoCheck       = CheckMode("")
	SilentCheck   = CheckMode("silent")
	ErrorCheck    = CheckMode("error")
	ValidateCheck = CheckMode("validate")
)

var ValidCheckModes = []CheckMode{
	NoCheck, SilentCheck, ErrorCheck, ValidateCheck,
}

func (c CheckMode) Validate() error {
	for _, checkMode := range ValidCheckModes {
		if c == checkMode {
			return nil
		}
	}
	return &errors.ValidationError{Msg: "checkMode is not allowed"}
}

type SyncState string

const SyncStateCleared = SyncState("__INTERNAL_CLEARED__")

func (s SyncState) Validate() error {
	// SyncTypeFull does not send any state :/
	return nil
}

type SyncType string

const SyncTypeFull = SyncType("")
const SyncTypeFullIncremental = SyncType("full")
const SyncTypeIncremental = SyncType("incremental")

var ValidSyncTypes = []SyncType{
	SyncTypeFull, SyncTypeFullIncremental, SyncTypeIncremental,
}

func (s SyncType) IsFull() bool {
	return s == SyncTypeFull || s == SyncTypeFullIncremental
}

func (s SyncType) Validate() error {
	for _, syncType := range ValidSyncTypes {
		if s == syncType {
			return nil
		}
	}
	return &errors.ValidationError{Msg: "syncType is not allowed"}
}

type RootResourcePath sharedTypes.PathName

const UnsafeRootResourcePathCharacters = "#&;`|*?~<>^()[]{}$\\\x0A\xFF\x00"

func (r RootResourcePath) ContainsUnsafeCharacters() bool {
	return strings.ContainsAny(string(r), UnsafeRootResourcePathCharacters)
}

func (r RootResourcePath) MakeSafe() (RootResourcePath, error) {
	if r.ContainsUnsafeCharacters() {
		withSafeCharacters := RootResourcePath(
			strings.Trim(string(r), UnsafeRootResourcePathCharacters),
		)
		return withSafeCharacters, nil
	}
	return r, nil
}

func (r RootResourcePath) Validate() error {
	if err := sharedTypes.PathName(r).Validate(); err != nil {
		return err
	}
	if r.ContainsUnsafeCharacters() {
		return &errors.ValidationError{Msg: "blocked characters in file/path"}
	}
	return nil
}

type ModifiedAt int64

func (m *ModifiedAt) Validate() error {
	if m == nil || *m == 0 {
		return &errors.ValidationError{Msg: "missing modified timestamp"}
	}
	return nil
}

func (m *ModifiedAt) String() string {
	if m == nil {
		return "0"
	}
	return sharedTypes.Int(*m).String()
}

// The Resource is either the inline doc Content,
//  or a file with download URL and ModifiedAt timestamp.
type Resource struct {
	Path       sharedTypes.PathName  `json:"path"`
	Content    *sharedTypes.Snapshot `json:"content,omitempty"`
	ModifiedAt *ModifiedAt           `json:"modified,omitempty"`
	URL        *sharedTypes.URL      `json:"url,omitempty"`
}

func (r *Resource) IsDoc() bool {
	return r.Content != nil
}

func (r *Resource) Validate() error {
	if err := r.Path.Validate(); err != nil {
		return err
	}
	if r.IsDoc() {
		// doc
	} else {
		// file
		if r.URL == nil {
			return &errors.ValidationError{Msg: "missing file url"}
		}
		if err := r.URL.Validate(); err != nil {
			return errors.Tag(err, "file url is invalid")
		}
		if r.ModifiedAt == nil {
			return &errors.ValidationError{
				Msg: "missing file modified timestamp",
			}
		}
		if err := r.ModifiedAt.Validate(); err != nil {
			return err
		}
	}
	return nil
}

type Resources []*Resource

func (r Resources) Validate() error {
	for _, resource := range r {
		if err := resource.Validate(); err != nil {
			return err
		}
	}
	return nil
}

type CompileOptions struct {
	Check        CheckMode     `json:"check"`
	Compiler     Compiler      `json:"compiler"`
	CompileGroup CompileGroup  `json:"compileGroup"`
	Draft        DraftModeFlag `json:"draft"`
	ImageName    ImageName     `json:"imageName"`
	SyncState    SyncState     `json:"syncState"`
	SyncType     SyncType      `json:"syncType"`
	Timeout      Timeout       `json:"timeout"`
}

func (c CompileOptions) Validate() error {
	if err := c.Check.Validate(); err != nil {
		return err
	}
	if err := c.Compiler.Validate(); err != nil {
		return err
	}
	if err := c.CompileGroup.Validate(); err != nil {
		return err
	}
	if err := c.Draft.Validate(); err != nil {
		return err
	}
	if err := c.ImageName.Validate(); err != nil {
		return err
	}
	if err := c.SyncState.Validate(); err != nil {
		return err
	}
	if err := c.SyncType.Validate(); err != nil {
		return err
	}
	if err := c.Timeout.Validate(); err != nil {
		return err
	}
	return nil
}

type CompileRequest struct {
	Options          CompileOptions   `json:"options"`
	Resources        Resources        `json:"resources"`
	RootResourcePath RootResourcePath `json:"rootResourcePath"`

	// Internal fields.
	RootDoc              *Resource `json:"-"`
	RootDocAliasResource *Resource `json:"-"`
}

func (c *CompileRequest) Preprocess() error {
	if c.RootResourcePath == "" {
		c.RootResourcePath = "main.tex"
	}
	if c.Options.Compiler == "" {
		c.Options.Compiler = PDFLatex
	}
	if c.Options.Timeout == 0 {
		// TODO: This is a bad default.
		c.Options.Timeout = MaxTimeout
	}
	if c.Options.Timeout < 1000 {
		// timeout is likely in seconds.
		c.Options.Timeout *= 1000_000_000
	}

	rootResourcePath := sharedTypes.PathName(c.RootResourcePath)
	var rootDoc *Resource
	for _, resource := range c.Resources {
		if resource.Path == rootResourcePath {
			rootDoc = resource
			break
		}
	}
	if rootDoc == nil {
		return &errors.ValidationError{Msg: "missing rootDoc resource"}
	}
	if !rootDoc.IsDoc() {
		return &errors.ValidationError{Msg: "rootDoc is not a doc"}
	}
	c.RootDoc = rootDoc

	safe, err := c.RootResourcePath.MakeSafe()
	if err != nil {
		return err
	}
	if safe != c.RootResourcePath {
		rootDoc.Path = sharedTypes.PathName(safe)
		c.RootResourcePath = safe
	}

	return nil
}

func (c *CompileRequest) Validate() error {
	if err := c.Options.Validate(); err != nil {
		return err
	}
	if err := c.Resources.Validate(); err != nil {
		return err
	}
	if err := c.RootResourcePath.Validate(); err != nil {
		return err
	}
	if c.RootDoc == nil {
		return &errors.ValidationError{Msg: "missing rootDoc resource"}
	}
	return nil
}

type DownloadPath string
type OutputFile struct {
	Build        BuildId              `json:"build"`
	DownloadPath DownloadPath         `json:"url"`
	Path         sharedTypes.PathName `json:"path"`
	Size         int64                `json:"size,omitempty"`
	Type         sharedTypes.FileType `json:"type"`
}
type OutputFiles []OutputFile

type Timings struct {
	FetchContent sharedTypes.Timed `json:"fetchContent"`
	Compile      sharedTypes.Timed `json:"compile"`
	CompileE2E   sharedTypes.Timed `json:"compileE2E"`
	Output       sharedTypes.Timed `json:"output"`
	Sync         sharedTypes.Timed `json:"sync"`
}

type CompileStatus string
type CompileError string
type CompileResponse struct {
	Status      CompileStatus `json:"status,omitempty"`
	Error       CompileError  `json:"error,omitempty"`
	OutputFiles OutputFiles   `json:"outputFiles"`
	Timings     Timings       `json:"timings"`
}
