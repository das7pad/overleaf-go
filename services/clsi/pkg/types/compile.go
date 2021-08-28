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
	"strconv"
	"strings"
	"time"

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

func (c Compiler) Validate(*Options) error {
	for _, compiler := range ValidCompilers {
		if c == compiler {
			return nil
		}
	}
	return &errors.ValidationError{Msg: "compiler not allowed"}
}

type DraftModeFlag bool

func (d DraftModeFlag) Validate(*Options) error {
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

func (c CheckMode) Validate(*Options) error {
	for _, checkMode := range ValidCheckModes {
		if c == checkMode {
			return nil
		}
	}
	return &errors.ValidationError{Msg: "checkMode is not allowed"}
}

type SyncState string

const SyncStateCleared = SyncState("__INTERNAL_CLEARED__")

func (s SyncState) Validate(*Options) error {
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

func (s SyncType) Validate(*Options) error {
	for _, syncType := range ValidSyncTypes {
		if s == syncType {
			return nil
		}
	}
	return &errors.ValidationError{Msg: "syncType is not allowed"}
}

type RootResourcePath FileName

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

func (r RootResourcePath) Validate(*Options) error {
	if err := FileName(r).Validate(); err != nil {
		return err
	}
	if r.ContainsUnsafeCharacters() {
		return &errors.ValidationError{Msg: "blocked characters in file/path"}
	}
	return nil
}

type Content string
type ModifiedAt int64

func (m *ModifiedAt) Validate(*Options) error {
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
	Path       FileName         `json:"path"`
	Content    *Content         `json:"content"`
	ModifiedAt *ModifiedAt      `json:"modified"`
	URL        *sharedTypes.URL `json:"url"`
}

func (r *Resource) IsDoc() bool {
	return r.Content != nil
}

func (r *Resource) Validate(options *Options) error {
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
		if err := r.ModifiedAt.Validate(options); err != nil {
			return err
		}
	}
	return nil
}

type Resources []*Resource

func (r Resources) Validate(options *Options) error {
	for _, resource := range r {
		if err := resource.Validate(options); err != nil {
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

func (c CompileOptions) Validate(options *Options) error {
	if err := c.Check.Validate(options); err != nil {
		return err
	}
	if err := c.Compiler.Validate(options); err != nil {
		return err
	}
	if err := c.CompileGroup.Validate(options); err != nil {
		return err
	}
	if err := c.Draft.Validate(options); err != nil {
		return err
	}
	if err := c.ImageName.Validate(options); err != nil {
		return err
	}
	if err := c.SyncState.Validate(options); err != nil {
		return err
	}
	if err := c.SyncType.Validate(options); err != nil {
		return err
	}
	if err := c.Timeout.Validate(options); err != nil {
		return err
	}
	return nil
}

type CompileRequest struct {
	Options          CompileOptions   `json:"options"`
	Resources        Resources        `json:"resources"`
	RootResourcePath RootResourcePath `json:"rootResourcePath"`

	// Internal fields.
	RootDoc              *Resource
	RootDocAliasResource *Resource
}

func (c *CompileRequest) Preprocess(options *Options) error {
	if c.RootResourcePath == "" {
		c.RootResourcePath = "main.tex"
	}
	if c.Options.CompileGroup == "" {
		c.Options.CompileGroup = options.DefaultCompileGroup
	}
	if c.Options.Compiler == "" {
		c.Options.Compiler = PDFLatex
	}
	if c.Options.ImageName == "" {
		c.Options.ImageName = options.DefaultImage
	}
	if c.Options.Timeout == 0 {
		// TODO: This is a bad default.
		c.Options.Timeout = MaxTimeout
	}
	if c.Options.Timeout < 1000 {
		// timeout is likely in seconds.
		c.Options.Timeout *= 1000_000_000
	}

	rootResourcePath := FileName(c.RootResourcePath)
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
		rootDoc.Path = FileName(safe)
		c.RootResourcePath = safe
	}

	return nil
}

func (c *CompileRequest) Validate(options *Options) error {
	if err := c.Options.Validate(options); err != nil {
		return err
	}
	if err := c.Resources.Validate(options); err != nil {
		return err
	}
	if err := c.RootResourcePath.Validate(options); err != nil {
		return err
	}
	if c.RootDoc == nil {
		return &errors.ValidationError{Msg: "missing rootDoc resource"}
	}
	return nil
}

type DownloadPath string
type FileType string
type OutputFile struct {
	Build        BuildId      `json:"build"`
	DownloadPath DownloadPath `json:"url"`
	Path         FileName     `json:"path"`
	Size         int64        `json:"size,omitempty"`
	Type         FileType     `json:"type"`
}
type OutputFiles []OutputFile

type Timed struct {
	t0   *time.Time
	diff time.Duration
}

func (t *Timed) Begin() {
	now := time.Now()
	t.t0 = &now
}

func (t *Timed) End() {
	if t.t0 == nil {
		return
	}
	t.diff = time.Now().Sub(*t.t0)
	t.t0 = nil
}

func (t *Timed) Diff() int64 {
	return t.diff.Milliseconds()
}

func (t *Timed) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatInt(t.Diff(), 10)), nil
}

func (t *Timed) UnmarshalJSON(bytes []byte) error {
	diff, err := strconv.ParseInt(string(bytes), 10, 64)
	if err != nil {
		return err
	}
	t.diff = time.Duration(diff * int64(time.Millisecond))
	return nil
}

type Timings struct {
	Compile    Timed `json:"compile"`
	CompileE2E Timed `json:"compileE2E"`
	Output     Timed `json:"output"`
	Sync       Timed `json:"sync"`
}

type CompileStatus string
type CompileError string
type CompileResponse struct {
	Status      CompileStatus `json:"status"`
	Error       CompileError  `json:"error"`
	OutputFiles OutputFiles   `json:"outputFiles"`
	Timings     Timings       `json:"timings"`
}
