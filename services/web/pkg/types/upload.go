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
	"io"
	"mime/multipart"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

const (
	MaxUploadSize = 50 * 1024 * 1024
)

type UploadFileRequest struct {
	ProjectId      primitive.ObjectID `json:"-"`
	UserId         primitive.ObjectID `json:"-"`
	ParentFolderId primitive.ObjectID `json:"-"`
	UploadDetails
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
