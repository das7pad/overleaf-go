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

package notification

import (
	"encoding/json"
	"time"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type IdField struct {
	Id sharedTypes.UUID `json:"_id" edgedb:"id"`
}

type KeyField struct {
	Key string `json:"key" edgedb:"key"`
}

type UserIdField struct {
	UserId sharedTypes.UUID `json:"user_id" edgedb:"user_id"`
}

type ExpiresField struct {
	Expires time.Time `json:"expires,omitempty" edgedb:"expires_at"`
}

type TemplateKeyField struct {
	TemplateKey string `json:"templateKey,omitempty" edgedb:"template_key"`
}

type MessageOptsField struct {
	MessageOptions json.RawMessage `json:"messageOpts,omitempty" edgedb:"message_options"`
}
