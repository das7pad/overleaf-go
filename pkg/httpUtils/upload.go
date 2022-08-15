// Golang port of Overleaf
// Copyright (C) 2021-2022 Jakob Ackermann <das7pad@outlook.com>
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

package httpUtils

import (
	"mime/multipart"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type UploadDetails struct {
	File     multipart.File
	FileName sharedTypes.Filename
	Size     int64
}

func (d *UploadDetails) Cleanup() {
	if f := d.File; f != nil {
		_ = f.Close()
	}
}

const multipartHeaderOverhead = 5 * 1024

func ProcessFileUpload(c *Context, sizeLimit, memoryLimit int64, d *UploadDetails) bool {
	if c.Request.ContentLength > sizeLimit+multipartHeaderOverhead {
		RespondErr(c, &errors.BodyTooLargeError{})
		return false
	}
	if err := c.Request.ParseMultipartForm(memoryLimit); err != nil {
		RespondErr(c, &errors.ValidationError{
			Msg: "cannot parse multipart form",
		})
		return false
	}

	//goland:noinspection SpellCheckingInspection
	fileHeaders := c.Request.MultipartForm.File["qqfile"]
	if len(fileHeaders) == 0 {
		RespondErr(c, &errors.ValidationError{Msg: "missing file"})
		return false
	}
	fh := fileHeaders[0]
	if fh.Size > sizeLimit {
		RespondErr(c, &errors.BodyTooLargeError{})
		return false
	}

	filename := sharedTypes.Filename(fh.Filename)
	if err := filename.Validate(); err != nil {
		RespondErr(c, err)
		return false
	}

	f, err := fh.Open()
	if err != nil {
		RespondErr(c, errors.Tag(err, "cannot open file"))
		return false
	}

	d.File = f
	d.FileName = filename
	d.Size = fh.Size
	return true
}
