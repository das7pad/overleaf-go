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
	"syscall"
)

const maxBatchSize = 4 * 1024 * 1024

func copySendFile(dest *os.File, src *os.File, size int) error {
	remaining := size
	for remaining > 0 {
		batchSize := remaining
		if batchSize > maxBatchSize {
			batchSize = maxBatchSize
		}
		n, err := syscall.Sendfile(
			int(dest.Fd()),
			int(src.Fd()),
			nil,
			batchSize,
		)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		remaining -= n
	}
	return nil
}
