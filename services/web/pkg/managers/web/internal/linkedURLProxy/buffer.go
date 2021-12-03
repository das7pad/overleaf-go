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

package linkedURLProxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type bufferedFile struct {
	fsPath   string
	tempFile *os.File
	size     int64
	path     sharedTypes.PathName
}

func (b *bufferedFile) Size() int64 {
	return b.size
}

func (b *bufferedFile) Path() sharedTypes.PathName {
	return b.path
}

func (b *bufferedFile) Open() (io.ReadCloser, error) {
	if f := b.tempFile; f != nil {
		// Give ownership on tempFile to caller.
		b.tempFile = nil
		return f, nil
	}
	return os.Open(b.fsPath)
}

func (b *bufferedFile) File() *os.File {
	return b.tempFile
}

func (b *bufferedFile) Cleanup() {
	if b.tempFile != nil {
		_ = b.tempFile.Close()
	}
	_ = os.Remove(b.fsPath)
}

// DownloadFile downloads a remote file via the proxy.
// The call-site must call the bufferedFile.Cleanup() method when finished.
func (m *manager) DownloadFile(ctx context.Context, src *sharedTypes.URL) (*bufferedFile, error) {
	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, chainURL(src, m.chain), nil,
	)
	if err != nil {
		return nil, errors.Tag(err, "cannot prepare http request")
	}
	res, err := m.client.Do(req)
	if err != nil {
		return nil, errors.Tag(err, "cannot send http request")
	}
	defer func() {
		_, _ = io.Discard.(io.ReaderFrom).ReadFrom(res.Body)
		_ = res.Body.Close()
	}()
	if res.StatusCode != 200 {
		switch res.StatusCode {
		case http.StatusUnprocessableEntity:
			return nil, &errors.UnprocessableEntityError{
				Msg: fmt.Sprintf(
					"upstream returned non success: %s",
					res.Header.Get("X-Upstream-Status-Code"),
				),
			}
		case http.StatusRequestEntityTooLarge:
			return nil, &errors.BodyTooLargeError{}
		default:
			return nil, errors.New(fmt.Sprintf(
				"proxy returned non success: %d", res.StatusCode,
			))
		}
	}
	f, err := os.CreateTemp("", "download-buffer")
	if err != nil {
		return nil, errors.Tag(
			err, "cannot open temp file for buffering",
		)
	}
	file := &bufferedFile{
		fsPath:   f.Name(),
		tempFile: f,
	}
	n, err := io.Copy(f, res.Body)
	if err != nil {
		file.Cleanup()
		return nil, errors.Tag(err, "cannot pipe file")
	}
	file.size = n
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		file.Cleanup()
		return nil, errors.Tag(err, "cannot seek buffer to start")
	}
	path := sharedTypes.PathName(src.FileNameFromPath())
	if err = path.Validate(); err == nil {
		file.path = path
	}
	return file, nil
}
