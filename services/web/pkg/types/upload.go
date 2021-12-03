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
	"fmt"
	"io"
	"mime/multipart"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/session"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
	"github.com/das7pad/overleaf-go/services/web/pkg/types/internal/conflictChecker"
)

const (
	MaxUploadSize  = 50 * 1024 * 1024
	maxProjectSize = 300 * 1024 * 1024
)

var errTotalSizeTooHigh = errors.Tag(
	&errors.BodyTooLargeError{}, "total size must be below 300MB",
)

type UploadFileRequest struct {
	ProjectId      primitive.ObjectID `json:"-"`
	UserId         primitive.ObjectID `json:"-"`
	ParentFolderId primitive.ObjectID `json:"-"`
	UploadDetails
}

type CreateProjectFromZipRequest struct {
	Session        *session.Session `json:"-"`
	AddHeader      AddHeaderFn      `json:"-"`
	HasDefaultName bool             `json:"-"`
	Name           project.Name     `json:"-"`
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
	Open() (io.ReadCloser, error)
}

type CreateProjectRequest struct {
	AddHeader      AddHeaderFn
	Compiler       clsiTypes.Compiler
	Files          []CreateProjectFile
	HasDefaultName bool
	Name           project.Name
	UserId         primitive.ObjectID
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
	if len(r.Files) == 0 {
		return &errors.ValidationError{Msg: "no files found"}
	}
	if len(r.Files) > 2000 {
		return &errors.ValidationError{Msg: "too many files"}
	}

	cc := make(conflictChecker.ConflictChecker, len(r.Files)*3)
	sum := int64(0)
	for _, file := range r.Files {
		path := file.Path()
		if err := path.Validate(); err != nil {
			return err
		}
		size := file.Size()
		if size > MaxUploadSize {
			return &errors.ValidationError{
				Msg: fmt.Sprintf("file %q is too large", path),
			}
		}
		sum += size
		if sum > maxProjectSize {
			return errTotalSizeTooHigh
		}
		if err := cc.RegisterFile(path); err != nil {
			return err
		}
	}
	return nil
}

type CreateProjectResponse struct {
	Success   bool                `json:"success"`
	Error     string              `json:"error,omitempty"`
	ProjectId *primitive.ObjectID `json:"project_id,omitempty"`
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
	if d.Size > MaxUploadSize {
		return &errors.BodyTooLargeError{}
	}
	return nil
}

func (d *UploadDetails) SeekFileToStart() error {
	_, err := d.File.Seek(0, io.SeekStart)
	if err != nil {
		return errors.Tag(err, "cannot seek to start")
	}
	return nil
}
