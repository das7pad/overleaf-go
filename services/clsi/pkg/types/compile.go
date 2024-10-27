// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/constants"
)

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

func (c CheckMode) Validate() error {
	switch c {
	case NoCheck, SilentCheck, ErrorCheck, ValidateCheck:
		return nil
	default:
		return &errors.ValidationError{Msg: "checkMode is not allowed"}
	}
}

type SyncState string

const SyncStateCleared = SyncState("")

func (s SyncState) Validate() error {
	if s == "" {
		return &errors.ValidationError{Msg: "missing sync state"}
	}
	return nil
}

type SyncType string

const (
	SyncTypeFullIncremental = SyncType("full")
	SyncTypeIncremental     = SyncType("incremental")
)

func (s SyncType) IsFull() bool {
	return s == SyncTypeFullIncremental
}

func (s SyncType) Validate() error {
	switch s {
	case SyncTypeFullIncremental, SyncTypeIncremental:
		return nil
	default:
		return &errors.ValidationError{Msg: "syncType is not allowed"}
	}
}

// The Resource is either the inline doc Content, or a file with download URL.
type Resource struct {
	Path    sharedTypes.PathName `json:"path"`
	Content string               `json:"content,omitempty"`
	URL     *sharedTypes.URL     `json:"url,omitempty"`
	Version sharedTypes.Version  `json:"v"`
}

func (r *Resource) IsDoc() bool {
	return r.URL == nil
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

type CompileOptionsHash string

type CompileOptions struct {
	Check            CheckMode                  `json:"check"`
	Compiler         sharedTypes.Compiler       `json:"compiler"`
	CompileGroup     sharedTypes.CompileGroup   `json:"compileGroup"`
	Draft            DraftModeFlag              `json:"draft"`
	ImageName        sharedTypes.ImageName      `json:"imageName"`
	RootResourcePath sharedTypes.PathName       `json:"rootResourcePath"`
	SyncState        SyncState                  `json:"syncState"`
	SyncType         SyncType                   `json:"syncType"`
	Timeout          sharedTypes.ComputeTimeout `json:"timeout"`
}

func (c CompileOptions) Hash() CompileOptionsHash {
	return CompileOptionsHash(fmt.Sprintf(
		"%s:%s:%s:%t:%s:%s:%s:%d",
		c.Check, c.Compiler, c.CompileGroup, c.Draft, c.ImageName,
		c.RootResourcePath, c.SyncState, c.Timeout,
	))
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
	if err := c.RootResourcePath.Validate(); err != nil {
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
	Options   CompileOptions `json:"options"`
	Resources Resources      `json:"resources"`

	// Internal fields.
	RootDoc              *Resource `json:"-"`
	RootDocAliasResource *Resource `json:"-"`
}

func (c *CompileRequest) Preprocess() error {
	if c.Options.RootResourcePath == "" {
		c.Options.RootResourcePath = "main.tex"
	}
	if c.Options.Compiler == "" {
		c.Options.Compiler = sharedTypes.PDFLaTeX
	}
	if c.Options.Timeout < 1000 {
		// timeout is likely in seconds.
		c.Options.Timeout *= sharedTypes.ComputeTimeout(time.Second)
	}

	rootResourcePath := c.Options.RootResourcePath
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
	return nil
}

func (c *CompileRequest) Validate() error {
	if err := c.Options.Validate(); err != nil {
		return err
	}
	if err := c.Resources.Validate(); err != nil {
		return err
	}
	if c.RootDoc == nil {
		return &errors.ValidationError{Msg: "missing rootDoc resource"}
	}
	return nil
}

type DownloadPath string

const publicProjectPrefix = "/project"

func BuildDownloadPath(projectId, userId sharedTypes.UUID, buildId BuildId, name sharedTypes.PathName) DownloadPath {
	return BuildDownloadPathFromNamespace(
		Namespace(projectId.Concat('-', userId)), buildId, name,
	)
}

func BuildDownloadPathFromNamespace(namespace Namespace, buildId BuildId, name sharedTypes.PathName) DownloadPath {
	return DownloadPath(
		publicProjectPrefix +
			"/" + string(namespace) +
			"/" + constants.CompileOutputLabel +
			"/" + string(buildId) +
			"/" + string(name),
	)
}

type OutputFile struct {
	Build        BuildId              `json:"build"`
	DownloadPath DownloadPath         `json:"url"`
	Path         sharedTypes.PathName `json:"path"`
	Size         int64                `json:"size,omitempty"`
	Type         sharedTypes.FileType `json:"type"`
	Ranges       []PDFCachingRange    `json:"ranges,omitempty"`
	ContentId    BuildId              `json:"contentId,omitempty"`
}
type OutputFiles []OutputFile

func (o OutputFiles) AddRanges(ranges []PDFCachingRange, contentId BuildId) {
	if len(ranges) == 0 {
		return
	}
	for i, file := range o {
		if file.Path == constants.OutputPDF {
			o[i].Ranges = ranges
			o[i].ContentId = contentId
			break
		}
	}
}

type Timings struct {
	FetchContent      sharedTypes.Timed `json:"fetchContent"`
	Sync              sharedTypes.Timed `json:"sync"`
	Setup             sharedTypes.Timed `json:"setup"`
	Compile           sharedTypes.Timed `json:"compile"`
	CompileUserTime   sharedTypes.Timed `json:"compileUserTime"`
	CompileSystemTime sharedTypes.Timed `json:"compileSystemTime"`
	Output            sharedTypes.Timed `json:"output"`
	PDFCaching        sharedTypes.Timed `json:"PDFCaching"`
	CompileE2E        sharedTypes.Timed `json:"compileE2E"`
}

type CompileStatus string

type CompileError string

type CompileResponse struct {
	Status      CompileStatus `json:"status,omitempty"`
	Error       CompileError  `json:"error,omitempty"`
	OutputFiles OutputFiles   `json:"outputFiles"`
	Timings     Timings       `json:"timings"`
}

type PDFCachingRange struct {
	ObjectId string           `json:"objectId"`
	Start    uint64           `json:"start"`
	End      uint64           `json:"end"`
	Hash     sharedTypes.Hash `json:"hash"`
}
