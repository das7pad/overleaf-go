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

package copyFile

import (
	"io"
	"os"
	"path"
	"syscall"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

func Atomic(dst string, reader io.Reader) error {
	return copyAtomic(dst, reader, 0, 0)
}

func AtomicN(dst string, reader io.Reader, mode os.FileMode, n int64) error {
	return copyAtomic(dst, reader, mode, n)
}

func AtomicMode(dst string, reader io.Reader, mode os.FileMode) error {
	return copyAtomic(dst, reader, mode, 0)
}

func copyAtomic(dst string, reader io.Reader, mode os.FileMode, n int64) error {
	writer, err := os.CreateTemp(path.Dir(dst), ".atomicWrite-*")
	if err != nil {
		return errors.Tag(err, "mktemp")
	}
	defer func() {
		if err != nil {
			_ = os.Remove(writer.Name())
		}
	}()
	if n > 0 {
		_, err = io.CopyN(writer, reader, n)
	} else {
		_, err = io.Copy(writer, reader)
	}
	if err != nil {
		_ = writer.Close()
		return errors.Tag(err, "copy")
	}
	if mode != 0 {
		if err = writer.Chmod(mode); err != nil {
			return errors.Tag(err, "chmod dst")
		}
	}
	if err = writer.Close(); err != nil {
		return errors.Tag(err, "close dst")
	}
	if err = syscall.Rename(writer.Name(), dst); err != nil {
		return errors.Tag(err, "rename")
	}
	return nil
}
