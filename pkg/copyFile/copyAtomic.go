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

package copyFile

import (
	"io"
	"os"
	"path"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

func Atomic(dest string, reader io.Reader) error {
	return copyAtomic(dest, reader, 0)
}

func AtomicWithMode(dest string, reader *os.File) error {
	stat, err := reader.Stat()
	if err != nil {
		return errors.Tag(err, "stat src")
	}
	return copyAtomic(dest, reader, stat.Mode())
}

func copyAtomic(dest string, reader io.Reader, mode os.FileMode) error {
	writer, err := os.CreateTemp(path.Dir(dest), ".atomicWrite-*")
	if err != nil {
		return errors.Tag(err, "mktemp")
	}
	defer func() {
		if err != nil {
			_ = os.Remove(writer.Name())
		}
	}()
	if _, err = io.Copy(writer, reader); err != nil {
		_ = writer.Close()
		return errors.Tag(err, "copy")
	}
	if mode != 0 {
		if err = writer.Chmod(mode); err != nil {
			return errors.Tag(err, "chmod dest")
		}
	}
	if err = writer.Close(); err != nil {
		return errors.Tag(err, "close dest")
	}
	if err = os.Rename(writer.Name(), dest); err != nil {
		return errors.Tag(err, "rename")
	}
	return nil
}
