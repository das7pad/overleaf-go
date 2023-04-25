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

package httpUtils

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type UploadDetails struct {
	File        multipart.File
	tmpFileName string
	FileName    sharedTypes.Filename
	Size        int64
}

func (d *UploadDetails) Cleanup() {
	if f := d.File; f != nil {
		_ = f.Close()
	}
	if d.tmpFileName != "" {
		_ = os.Remove(d.tmpFileName)
	}
}

func ProcessFileUpload(d *UploadDetails, c *Context, memoryLimit int64) bool {
	if err := tryProcessFileUpload(d, c, memoryLimit); err != nil {
		RespondErr(c, err)
		return false
	}
	return true
}

func parseFilenameFromCD(cd string) (sharedTypes.Filename, error) {
	_, params, err := mime.ParseMediaType(cd)
	if err != nil {
		return "", &errors.ValidationError{Msg: "invalid Content-Disposition"}
	}
	name := sharedTypes.Filename(params["filename"])
	if err = name.Validate(); err != nil {
		return "", errors.Tag(err, "invalid Content-Disposition")
	}
	return name, nil
}

func tryProcessFileUpload(d *UploadDetails, c *Context, memoryLimit int64) error {
	if err := validateContentLength(c); err != nil {
		return err
	}
	name, err := parseFilenameFromCD(c.Request.Header.Get("Content-Disposition"))
	if err != nil {
		return err
	}
	d.FileName = name

	r := http.MaxBytesReader(c.Writer, c.Request.Body, c.Request.ContentLength)
	if c.Request.ContentLength <= memoryLimit {
		buf := bytes.Buffer{}
		if d.Size, err = io.Copy(&buf, r); err != nil {
			return errors.Tag(err, "copy body")
		}
		d.File = &bufferedFile{bytes.NewReader(buf.Bytes())}
	} else {
		var f *os.File
		if f, err = os.CreateTemp("", "upload"); err != nil {
			return errors.Tag(err, "create tmp file")
		}
		d.File = f
		d.tmpFileName = f.Name()
		if d.Size, err = io.Copy(f, r); err != nil {
			return errors.Tag(err, "copy body")
		}
	}
	return nil
}

type bufferedFile struct {
	*bytes.Reader
}

func (*bufferedFile) Close() error {
	return nil
}
