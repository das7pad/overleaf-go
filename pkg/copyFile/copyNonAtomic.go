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
)

func NonAtomic(dst string, src string) error {
	reader, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		_ = reader.Close()
	}()

	writer, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		_ = writer.Close()
	}()

	_, err = io.Copy(writer, reader)
	return err
}
