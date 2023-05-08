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

package linkedURLProxy

import (
	"context"
	"io"
	"os"
	"syscall"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
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

func (b *bufferedFile) PreComputedHash() sharedTypes.Hash {
	return ""
}

func (b *bufferedFile) SourceElement() project.TreeElement {
	return nil
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

func (b *bufferedFile) Move(target string) error {
	if b.tempFile != nil {
		_ = b.tempFile.Close()
	}
	return syscall.Rename(b.fsPath, target)
}

func (b *bufferedFile) ToUploadDetails() types.UploadDetails {
	return types.UploadDetails{
		File:     b.File(),
		FileName: b.Path().Filename(),
		Size:     b.Size(),
	}
}

// DownloadFile downloads a remote file via the proxy.
// The call-site must call the bufferedFile.Cleanup() method when finished.
func (m *manager) DownloadFile(ctx context.Context, src *sharedTypes.URL) (*bufferedFile, error) {
	body, cleanup, err := m.Fetch(ctx, src)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	f, err := os.CreateTemp("", "download-buffer")
	if err != nil {
		return nil, errors.Tag(
			err, "open temp file for buffering",
		)
	}
	file := bufferedFile{
		fsPath:   f.Name(),
		tempFile: f,
	}
	n, err := io.Copy(f, body)
	if err != nil {
		file.Cleanup()
		return nil, errors.Tag(err, "pipe file")
	}
	file.size = n
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		file.Cleanup()
		return nil, errors.Tag(err, "seek buffer to start")
	}
	path := sharedTypes.PathName(src.FileNameFromPath())
	if err = path.Validate(); err == nil {
		file.path = path
	}
	return &file, nil
}
