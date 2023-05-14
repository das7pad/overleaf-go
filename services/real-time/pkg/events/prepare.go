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

package events

import (
	"encoding/json"

	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

func mustPrepareBulkMessageOffline(response *types.RPCResponse, payload interface{}) types.WriteQueueEntry {
	if payload != nil {
		blob, err := json.Marshal(payload)
		if err != nil {
			panic(err)
		}
		response.Body = blob
	}
	entry, err := types.PrepareBulkMessage(response)
	if err != nil {
		panic(err)
	}
	return entry
}
