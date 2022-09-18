// Golang port of Overleaf
// Copyright (C) 2022 Jakob Ackermann <das7pad@outlook.com>
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

package flush

import (
	"time"

	"github.com/das7pad/overleaf-go/pkg/models/docHistory"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/sharejs/types/text"
)

func mergeComponents(a, b sharedTypes.Component) (bool, sharedTypes.Component) {
	if a.IsInsertion() {
		if b.IsInsertion() {
			if a.Position <= b.Position &&
				a.Position+len(a.Insertion) >= b.Position {
				a.Insertion = text.InjectInPlace(
					a.Insertion,
					b.Position-a.Position,
					b.Insertion,
				)
				return true, a
			} else {
				return false, a
			}
		} else if b.IsDeletion() {
			if a.Position <= b.Position &&
				a.Position+len(a.Insertion) >= b.Position+len(b.Deletion) {
				// The deletion is fully contained in the insertion.
				cutFrom := b.Position - a.Position
				s := a.Insertion[:len(a.Insertion)-len(b.Deletion)]
				copy(s[cutFrom:], a.Insertion[cutFrom+len(b.Deletion):])
				a.Insertion = s
				return true, a
			} else if a.Position >= b.Position &&
				a.Position+len(a.Insertion) <= b.Position+len(b.Deletion) {
				// The insertion is fully contained in the deletion.
				cutFrom := a.Position - b.Position
				s := b.Deletion[:len(b.Deletion)-len(a.Insertion)]
				copy(s[cutFrom:], b.Deletion[cutFrom+len(a.Insertion):])
				a.Deletion = s
				a.Insertion = nil
				a.Position = b.Position
				return true, a
			} else {
				return false, a
			}
		} else {
			// `b` is no-op
			return true, a
		}
	} else if a.IsDeletion() {
		if b.IsInsertion() {
			if a.Position == b.Position && a.Deletion.Equals(b.Insertion) {
				a.Deletion = sharedTypes.Snippet{}
				return true, a
			} else if a.Position == b.Position &&
				a.Position+len(a.Deletion) >= b.Position+len(b.Insertion) &&
				a.Deletion[:len(b.Insertion)].Equals(b.Insertion) {
				// The insertion is a prefix of the deletion.
				a.Deletion = a.Deletion[len(b.Insertion):]
				a.Position += len(b.Insertion)
				return true, a
			} else if b.Position ==
				a.Position+len(a.Deletion)-len(b.Insertion) &&
				a.Position+len(a.Deletion) >= b.Position+len(b.Insertion) &&
				len(a.Deletion)-len(b.Insertion) > 0 &&
				a.Deletion[len(a.Deletion)-len(b.Insertion):].
					Equals(b.Insertion) {
				// The insertion is a suffix of the deletion.
				end := len(a.Deletion) - len(b.Insertion)
				a.Deletion = a.Deletion[:end]
				return true, a
			} else if a.Position == b.Position &&
				len(a.Deletion) < len(b.Insertion) &&
				a.Deletion.Equals(b.Insertion[:len(a.Deletion)]) {
				// The deletion is a prefix of the insertion
				a.Insertion = b.Insertion[len(a.Deletion):]
				a.Deletion = nil
				return true, a
			} else if a.Position == b.Position &&
				len(a.Deletion) < len(b.Insertion) &&
				a.Deletion.
					Equals(b.Insertion[len(b.Insertion)-len(a.Deletion):]) {
				// The deletion is a suffix of the insertion
				end := len(b.Insertion) - len(a.Deletion)
				a.Deletion = nil
				a.Insertion = b.Insertion[:end]
				return true, a
			} else {
				// With the help of diff match and patch we could split a/b.
				return false, a
			}
		} else if b.IsDeletion() {
			if b.Position <= a.Position &&
				b.Position+len(b.Deletion) >= a.Position {
				a.Deletion = text.InjectInPlace(
					b.Deletion, a.Position-b.Position, a.Deletion,
				)
				a.Position = b.Position
				return true, a
			} else {
				return false, a
			}
		} else {
			// `b` is no-op
			return true, a
		}
	} else {
		// `a` is no-op
		return true, b
	}
}

func mergeUpdates(updates []sharedTypes.DocumentUpdate) []docHistory.ForInsert {
	totalComponents := 0
	for i := 0; i < len(updates); i++ {
		totalComponents += len(updates[i].Op)
	}

	dhSingle := make([]docHistory.ForInsert, 0, totalComponents)
	dhSingle = append(dhSingle, docHistory.ForInsert{
		UserId:  updates[0].Meta.UserId,
		Version: updates[0].Version,
		StartAt: updates[0].Meta.Timestamp.ToTime(),
		EndAt:   updates[0].Meta.Timestamp.ToTime(),
		Op: sharedTypes.Op{
			updates[0].Op[0],
		},
	})
	updates[0].Op = updates[0].Op[1:]
	for _, update := range updates {
		t := update.Meta.Timestamp.ToTime()
		for _, secondC := range update.Op {
			tail := &dhSingle[len(dhSingle)-1]
			firstC := tail.Op[0]
			if tail.UserId != update.Meta.UserId {
				// Do not mergeComponents updates from different users.
			} else if t.Sub(tail.EndAt) > time.Minute {
				// Too much time has passed between the two updates.
			} else if merged, c := mergeComponents(firstC, secondC); merged {
				// The two components have been merged, update meta data.
				tail.EndAt = t
				tail.Op[0] = c
				tail.Version = update.Version
				continue
			} else {
				// The two components are not connected.
			}
			dhSingle = append(dhSingle, docHistory.ForInsert{
				UserId:  update.Meta.UserId,
				Version: update.Version,
				StartAt: t,
				EndAt:   t,
				Op:      sharedTypes.Op{secondC},
			})
		}
	}
	return dhSingle
}

func mergeInserts(dhSingle []docHistory.ForInsert) []docHistory.ForInsert {
	dh := make([]docHistory.ForInsert, 0, len(dhSingle))
	var maxVersion sharedTypes.Version
	for _, update := range dhSingle {
		maxVersion = update.Version
		if update.Op[0].IsNoOp() {
			continue
		}
		if len(dh) == 0 {
			dh = append(dh, update)
			continue
		}
		tail := &dh[len(dh)-1]
		if update.UserId == tail.UserId &&
			(update.Version == tail.Version ||
				update.StartAt.Sub(tail.EndAt) <= time.Minute) {
			tail.EndAt = update.EndAt
			tail.Version = update.Version
			tail.Op = append(tail.Op, update.Op[0])
		} else {
			dh = append(dh, update)
		}
	}

	// Ensure that we have at least one entry for transporting the maxVersion.
	// We need to persist the maxVersion for detecting history jumps.
	if len(dh) == 0 {
		for _, update := range dhSingle {
			if update.Op[0].IsNoOp() {
				dh = dh[:1]
				dh[0] = update
			}
		}
	}
	dh[len(dh)-1].Version = maxVersion
	return dh
}
