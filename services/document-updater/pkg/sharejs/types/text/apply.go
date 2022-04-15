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

package text

import (
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func Apply(snapshot sharedTypes.Snapshot, ops sharedTypes.Op) (sharedTypes.Snapshot, error) {
	for _, op := range ops {
		if op.IsInsertion() {
			snapshot = InjectInPlace(snapshot, op.Position, op.Insertion)
		} else if op.IsDeletion() {
			start := op.Position
			end := op.Position + len(op.Deletion)
			deletionActual := snapshot.Slice(start, end)
			if string(op.Deletion) != string(deletionActual) {
				return nil, &errors.CodedError{
					Description: "Delete component '" +
						string(op.Deletion) +
						"' does not match deleted text '" +
						string(deletionActual) +
						"'",
				}
			}
			s := snapshot[:]
			snapshot = snapshot[:len(snapshot)-len(op.Deletion)]
			copy(snapshot[start:], s[end:])
		} else if op.IsComment() {
			start := op.Position
			end := op.Position + len(op.Comment)
			commentActual := snapshot.Slice(start, end)
			if string(op.Comment) != string(commentActual) {
				return nil, &errors.CodedError{
					Description: "Comment component '" +
						string(op.Comment) +
						"' does not match commented text '" +
						string(commentActual) +
						"'",
				}
			}
		}
		// NOTE: else case is handled in validation on ingestion into service.
	}
	return snapshot, nil
}
