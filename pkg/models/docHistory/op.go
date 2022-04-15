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

package docHistory

import (
	"encoding/json"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type OpWrapper struct {
	sharedTypes.Op
}

func (w *OpWrapper) HasBigDelete() bool {
	for _, component := range w.Op {
		if len(component.Deletion) > 16 {
			return true
		}
	}
	return false
}

func (w *OpWrapper) UnmarshalJSON(bytes []byte) error {
	return json.Unmarshal(bytes, &w.Op)
}

func (w *OpWrapper) MarshalJSON() ([]byte, error) {
	return json.Marshal(w.Op)
}
