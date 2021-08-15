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
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"

	"github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
)

var dmp = diffmatchpatch.New()

func init() {
	dmp.DiffTimeout = 100 * time.Millisecond
}

func Diff(before, after types.Snapshot) types.Op {
	diffs := dmp.DiffMainRunes(before, after, false)
	diffs = dmp.DiffCleanupSemantic(diffs)

	op := make(types.Op, 0, len(diffs))
	pos := 0
	for _, diff := range diffs {
		s := types.Snippet(diff.Text)
		switch diff.Type {
		case diffmatchpatch.DiffInsert:
			op = append(op, types.Component{
				Insertion: s,
				Position:  pos,
			})
			pos += len(s)
		case diffmatchpatch.DiffDelete:
			op = append(op, types.Component{
				Deletion: s,
				Position: pos,
			})
		case diffmatchpatch.DiffEqual:
			pos += len(s)
		}
	}
	return op
}
