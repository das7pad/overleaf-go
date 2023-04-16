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

package text

import (
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func Inject(s1 []rune, position int, s2 []rune) []rune {
	s := make([]rune, len(s1)+len(s2))
	copy(s, s1[:position])
	copy(s[position:], s2)
	copy(s[position+len(s2):], s1[position:])
	return s
}

func InjectInPlace(s1 []rune, position int, s2 []rune) []rune {
	var s []rune
	if newLen := len(s1) + len(s2); cap(s1) >= newLen {
		s = s1[:newLen]
	} else {
		s = make([]rune, newLen)
		copy(s, s1[:position])
	}
	copy(s[position+len(s2):], s1[position:])
	copy(s[position:], s2)
	return s
}

func appendOp(op sharedTypes.Op, c sharedTypes.Component) sharedTypes.Op {
	if len(op) == 0 {
		return append(op, c)
	}

	lastPos := len(op) - 1
	lastC := op[lastPos]
	if c.IsInsertion() {
		if lastC.IsInsertion() && overlapsByN(lastC, c, len(c.Insertion)) {
			op[lastPos].Insertion = Inject(
				lastC.Insertion, c.Position-lastC.Position, c.Insertion,
			)
			return op
		}
	} else if c.IsDeletion() {
		if lastC.IsDeletion() && overlapsByN(c, lastC, len(c.Deletion)) {
			op[lastPos].Deletion = Inject(
				c.Deletion, lastC.Position-c.Position, lastC.Deletion,
			)
			op[lastPos].Position = c.Position
			return op
		}
	}
	return append(op, c)
}

func overlapsByN(a, b sharedTypes.Component, n int) bool {
	return a.Position <= b.Position &&
		b.Position <= a.Position+n
}
