// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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

package sharedTypes

import (
	"encoding/json"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type EditorEventsMessage struct {
	RoomId      UUID            `json:"room_id"`
	Message     string          `json:"message"`
	Payload     json.RawMessage `json:"payload"`
	ProcessedBy string          `json:"processedBy,omitempty"`
	Source      PublicId        `json:"source,omitempty"`
}

func (m *EditorEventsMessage) MarshalJSON() ([]byte, error) {
	o := make([]byte, 0, len(`{"room_id":"00000000-0000-0000-0000-000000000000","message":,"payload":,"processed_by":,"source":}`)+len(m.Message)+len(m.Payload)+len(m.ProcessedBy)+len(m.Source))
	o = append(o, `{"room_id":"`...)
	o = m.RoomId.Append(o)
	o = append(o, `","message":"`...)
	o = append(o, m.Message...)
	o = append(o, `","payload":`...)
	o = append(o, m.Payload...)
	if len(m.ProcessedBy) > 0 {
		o = append(o, `,"processedBy":"`...)
		o = append(o, m.ProcessedBy...)
		o = append(o, '"')
	}
	if len(m.Source) > 0 {
		o = append(o, `,"source":"`...)
		o = append(o, m.Source...)
		o = append(o, '"')
	}
	o = append(o, '}')
	return o, nil
}

func (m *EditorEventsMessage) ChannelId() UUID {
	return m.RoomId
}

func (m *EditorEventsMessage) Validate() error {
	if m.Message == "" {
		return &errors.ValidationError{Msg: "missing message"}
	}
	return nil
}
