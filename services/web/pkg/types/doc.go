// Golang port of Overleaf
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

package types

import (
	"encoding/json"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Snapshot []rune

func (s *Snapshot) UnmarshalJSON(bytes []byte) error {
	var raw string
	if err := json.Unmarshal(bytes, &raw); err != nil {
		return err
	}
	*s = Snapshot(raw)
	return nil
}

func (s Snapshot) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}

type Version int64

type DocContentSnapshot struct {
	Id       primitive.ObjectID   `json:"_id"`
	Snapshot Snapshot             `json:"snapshot"`
	PathName sharedTypes.PathName `json:"pathname"`
	Version  Version              `json:"v"`
}

func (d *DocContentSnapshot) CheckIsValidRootDoc() error {
	return nil
}
