// Golang port of Overleaf
// Copyright (C) 2024 Jakob Ackermann <das7pad@outlook.com>
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

package randQueue

import (
	"crypto/rand"
	"log"
	"time"
)

type Q8 chan [8]byte

func (q Q8) Run(n int) {
	buf := make([]byte, n)
	for {
		if _, err := rand.Read(buf); err != nil {
			log.Printf("reading from rand.Read failed: %s", err)
			time.Sleep(time.Second)
			continue
		}
		for i := 0; i < n; i = i + 8 {
			q <- [8]byte(buf[i : i+8])
		}
	}
}

type Q16 chan [16]byte

func (q Q16) Run(n int) {
	buf := make([]byte, n)
	for {
		if _, err := rand.Read(buf); err != nil {
			log.Printf("reading from rand.Read failed: %s", err)
			time.Sleep(time.Second)
			continue
		}
		for i := 0; i < n; i = i + 16 {
			q <- [16]byte(buf[i : i+16])
		}
	}
}
