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

package copyFile

import (
	"os"
)

func NonAtomic(dest string, src string) error {
	reader, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		_ = reader.Close()
	}()

	writer, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() {
		_ = writer.Close()
	}()

	stat, err := reader.Stat()
	if err != nil {
		return err
	}
	size := int(stat.Size())

	return copySendFile(writer, reader, size)
}
