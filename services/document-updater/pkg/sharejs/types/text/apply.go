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
	"fmt"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func Apply(snapshot sharedTypes.Snapshot, ops sharedTypes.Op) (sharedTypes.Snapshot, error) {
	for _, op := range ops {
		if op.IsInsertion() {
			snapshot = InjectInPlace(snapshot, op.Position, op.Insertion)
			continue
		}

		// NOTE: validation on ingestion ensures `op` is a deletion.
		start := op.Position
		end := op.Position + len(op.Deletion)
		deletionActual := snapshot.Slice(start, end)
		if !op.Deletion.Equals(deletionActual) {
			return nil, &errors.CodedError{
				Description: fmt.Sprintf(
					"Delete component %q does not match deleted text %q",
					string(op.Deletion), string(deletionActual),
				),
			}
		}
		s := snapshot[:]
		snapshot = snapshot[:len(snapshot)-len(op.Deletion)]
		copy(snapshot[start:], s[end:])
	}
	return snapshot, nil
}
