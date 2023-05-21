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

package asyncForm

import (
	"github.com/das7pad/overleaf-go/pkg/errors"
)

type MessageType string

const (
	Error MessageType = "error"
)

type Message struct {
	Text string      `json:"text"`
	Type MessageType `json:"type"`
	Key  string      `json:"key,omitempty"`
}

type Response struct {
	Message    *Message `json:"message,omitempty"`
	RedirectTo string   `json:"redirectTo,omitempty"`
}

func (r *Response) SetCustomFormMessage(key string, err error) {
	r.Message = &Message{
		Key: key,
	}
	if err != nil {
		r.Message.Text = errors.GetPublicMessage(err, "internal server error")
		r.Message.Type = Error
	}
}
