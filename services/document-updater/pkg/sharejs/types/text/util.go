// Golang port of the Overleaf document-updater service
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

package text

import (
	"github.com/das7pad/document-updater/pkg/types"
)

func inject(s1 string, position int64, s2 string) string {
	return s1[:position] + s2 + s1[position:]
}

func appendOp(op types.Op, c types.Component) types.Op {
	if len(op) == 0 {
		return append(op, c)
	}

	lastPos := len(op) - 1
	lastC := op[lastPos]
	if c.IsInsertion() {
		if lastC.IsInsertion() && overlapsByN(lastC, c, len(c.Insertion)) {
			op[lastPos].Insertion = inject(
				lastC.Insertion, c.Position-lastC.Position, c.Insertion,
			)
			return op
		}
	} else if c.IsDeletion() {
		if lastC.IsDeletion() && overlapsByN(c, lastC, len(c.Deletion)) {
			op[lastPos].Deletion = inject(
				c.Deletion, lastC.Position-c.Position, lastC.Deletion,
			)
			op[lastPos].Position = c.Position
			return op
		}
	}
	return append(op, c)
}

func overlapsByN(a, b types.Component, n int) bool {
	return a.Position <= b.Position &&
		b.Position <= a.Position+int64(n)
}
