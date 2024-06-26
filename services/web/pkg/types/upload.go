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
	"io"
	"mime/multipart"

	"github.com/das7pad/overleaf-go/pkg/constants"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	spellingTypes "github.com/das7pad/overleaf-go/services/spelling/pkg/types"
	"github.com/das7pad/overleaf-go/services/web/pkg/types/internal/conflictChecker"
)

type UploadFileRequest struct {
	ProjectId      sharedTypes.UUID        `json:"-"`
	UserId         sharedTypes.UUID        `json:"-"`
	ParentFolderId sharedTypes.UUID        `json:"-"`
	LinkedFileData *project.LinkedFileData `json:"-"`
	ClientId       sharedTypes.PublicId    `json:"-"`
	UploadDetails
}

type CreateProjectFromZipRequest struct {
	WithSession
	AddHeader      AddHeaderFn          `json:"-"`
	Compiler       sharedTypes.Compiler `json:"-"`
	HasDefaultName bool                 `json:"-"`
	Name           project.Name         `json:"-"`
	UploadDetails
}

func (r *CreateProjectFromZipRequest) Preprocess() {
	if r.Name == "" && r.FileName != "" {
		r.HasDefaultName = true
		r.Name = project.Name(r.FileName.Basename())
	}
}

func (r *CreateProjectFromZipRequest) Validate() error {
	if err := r.Name.Validate(); err != nil {
		return err
	}
	if err := r.UploadDetails.Validate(); err != nil {
		return err
	}
	return nil
}

type CreateProjectFileWithCleanup interface {
	CreateProjectFile
	Cleanup()
}

type AddHeaderFn = func(s sharedTypes.Snapshot) sharedTypes.Snapshot

type CreateProjectFile interface {
	Size() int64
	Path() sharedTypes.PathName
	Open() (io.ReadCloser, bool, error)
	PreComputedHash() sharedTypes.Hash
	SourceElement() project.TreeElement
}

type CreateProjectRequest struct {
	AddHeader          AddHeaderFn
	Compiler           sharedTypes.Compiler
	ExtraFolders       []sharedTypes.DirName
	Files              []CreateProjectFile
	HasDefaultName     bool
	ImageName          sharedTypes.ImageName
	Name               project.Name
	RootDocPath        sharedTypes.PathName
	SourceProjectId    sharedTypes.UUID
	SpellCheckLanguage spellingTypes.SpellCheckLanguage
	UserId             sharedTypes.UUID
}

func (r *CreateProjectRequest) Validate() error {
	if r.UserId.IsZero() {
		return errors.New("must be logged in")
	}
	if r.Compiler != "" {
		if err := r.Compiler.Validate(); err != nil {
			return err
		}
	}
	if err := r.Name.Validate(); err != nil {
		return err
	}
	if len(r.Files) == 0 && len(r.ExtraFolders) == 0 {
		return &errors.ValidationError{Msg: "no files/folders found"}
	}
	if len(r.Files)+len(r.ExtraFolders) > constants.MaxFilesPerProject {
		return &errors.ValidationError{Msg: "too many files for new project"}
	}

	cc := make(conflictChecker.ConflictChecker, len(r.Files)*3)
	sum := int64(0)
	for _, file := range r.Files {
		path := file.Path()
		if err := path.Validate(); err != nil {
			return err
		}
		size := file.Size()
		if size > constants.MaxUploadSize {
			return &errors.ValidationError{
				Msg: fmt.Sprintf("file %q is too large", path),
			}
		}
		sum += size
		if sum > constants.MaxProjectSize {
			return errors.Tag(
				&errors.BodyTooLargeError{}, "total size must be below 300MB",
			)
		}
		if err := cc.RegisterFile(path); err != nil {
			return err
		}
	}
	for _, path := range r.ExtraFolders {
		if err := path.Validate(); err != nil {
			return err
		}
		if err := cc.RegisterFolder(path); err != nil {
			return err
		}
	}
	return nil
}

type CreateProjectResponse struct {
	Success   bool              `json:"success"`
	Error     string            `json:"error,omitempty"`
	ProjectId *sharedTypes.UUID `json:"project_id,omitempty"`
	Name      project.Name      `json:"name,omitempty"`
}

type UploadDetails struct {
	File     multipart.File       `json:"-"`
	FileName sharedTypes.Filename `json:"-"`
	Size     int64                `json:"-"`
}

func (d *UploadDetails) Validate() error {
	if err := d.FileName.Validate(); err != nil {
		return err
	}
	if d.Size > constants.MaxUploadSize {
		return &errors.BodyTooLargeError{}
	}
	return nil
}

func (d *UploadDetails) SeekFileToStart() error {
	_, err := d.File.Seek(0, io.SeekStart)
	if err != nil {
		return errors.Tag(err, "seek to start")
	}
	return nil
}
