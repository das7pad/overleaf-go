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

package session

import (
	"encoding/json"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type sessionValidationToken string

type sessionDataWithDeSyncProtection struct {
	ValidationToken sessionValidationToken `json:"validationToken"`
	Data
}

var (
	RedisConnectionDeSyncErr = errors.New("redis connection de-synced")
)

func (s sessionValidationToken) Validate(id Id) error {
	if s != id.toSessionValidationToken() {
		return RedisConnectionDeSyncErr
	}
	return nil
}

func serializeSession(id Id, data Data) ([]byte, error) {
	return json.Marshal(sessionDataWithDeSyncProtection{
		ValidationToken: id.toSessionValidationToken(),
		Data:            data,
	})
}

func deSerializeSession(id Id, blob []byte) (*Data, error) {
	dst := sessionDataWithDeSyncProtection{}
	if err := json.Unmarshal(blob, &dst); err != nil {
		return nil, err
	}
	if err := dst.ValidationToken.Validate(id); err != nil {
		return nil, err
	}
	return &dst.Data, nil
}
