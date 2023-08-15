// Golang port of Overleaf
// Copyright (C) 2023 Jakob Ackermann <das7pad@outlook.com>
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

package vlq

const (
	base64 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
)

func Encode(b []byte, value int) []byte {
	if value == 0 {
		// Hot path for source-maps
		return append(b, 'A')
	}
	if value == 1 {
		// Hot path for source-maps
		return append(b, 'C')
	}

	// Shift and set sign bit
	if value < 0 {
		value = -value<<1 | 1
	} else {
		value = value<<1 | 0
	}

	// value==0 is handled above via hot-path
	for value > 0 {
		// Take the first 5 bits
		field := value & (1<<5 - 1)
		value = value >> 5

		if value > 0 {
			// Set continuation bit in case there are remaining bits
			field = field | 1<<5
		}

		b = append(b, base64[field])
	}
	return b
}
