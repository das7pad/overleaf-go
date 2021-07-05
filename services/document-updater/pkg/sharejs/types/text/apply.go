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
	"github.com/das7pad/document-updater/pkg/errors"
	"github.com/das7pad/document-updater/pkg/types"
)

func Apply(snapshot types.Snapshot, ops types.Op) (types.Snapshot, error) {
	for _, op := range ops {
		if op.IsInsertion() {
			snapshot = types.Snapshot(
				inject(string(snapshot), op.Position, op.Insertion),
			)
		} else if op.IsDeletion() {
			start := op.Position
			end := op.Position + int64(len(op.Deletion))
			deletionActual := snapshot.Slice(start, end)
			if op.Deletion != deletionActual {
				return "", &errors.JavaScriptError{
					Message: "Delete component '" +
						op.Deletion +
						"' does not match deleted text '" +
						deletionActual +
						"'",
				}
			}
			snapshot = snapshot[:start] + snapshot[end:]
		} else if op.IsComment() {
			start := op.Position
			end := op.Position + int64(len(op.Comment))
			commentActual := snapshot.Slice(start, end)
			if op.Comment != commentActual {
				return "", &errors.JavaScriptError{
					Message: "Comment component '" +
						op.Comment +
						"' does not match commented text '" +
						commentActual +
						"'",
				}
			}
		}
		// NOTE: else case is handled in validation on ingestion into service.
	}
	return snapshot, nil
}
