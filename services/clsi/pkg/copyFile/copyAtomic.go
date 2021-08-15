// Golang port of the Overleaf clsi service
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

package copyFile

import (
	"io"
	"os"
	"path"

	"github.com/das7pad/clsi/pkg/errors"
)

func Atomic(reader io.Reader, dest string, copyMode bool) error {
	readerAsFile, readerIsFile := reader.(*os.File)
	var stat os.FileInfo
	if readerIsFile {
		var errStat error
		if stat, errStat = readerAsFile.Stat(); errStat != nil {
			return errStat
		}
	} else if copyMode {
		return errors.New("cannot action copyMode from non file")
	}

	dir := path.Dir(dest)
	writer, err := os.CreateTemp(dir, ".atomicWrite-*")
	if err != nil {
		return err
	}
	atomicWritePath := writer.Name()

	var errCopy error
	if readerIsFile {
		size := int(stat.Size())
		errCopy = copySendFile(writer, readerAsFile, size)
	} else {
		_, errCopy = io.Copy(writer, reader)
	}
	errClose := writer.Close()
	var errRename error
	var errPerms error
	if errCopy == nil && errClose == nil {
		if readerIsFile && copyMode {
			errPerms = os.Chmod(atomicWritePath, stat.Mode())
		}
		if errPerms == nil {
			errRename = os.Rename(atomicWritePath, dest)
			if errRename == nil {
				// Happy path.
				return nil
			}
		}
	}
	_ = os.Remove(atomicWritePath)
	if errCopy != nil {
		return errCopy
	}
	if errClose != nil {
		return errClose
	}
	if errPerms != nil {
		return errPerms
	}
	return errRename
}
