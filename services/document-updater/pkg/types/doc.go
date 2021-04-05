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

package types

import (
	"encoding/json"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/document-updater/pkg/errors"
)

const megabytes = 1024 * 1024
const MaxRangesSize = 3 * megabytes

type UnFlushedTime int64

type LastUpdatedCtx struct {
	At int64               `json:"at"`
	By *primitive.ObjectID `json:"by,omitempty"`
}

type ProjectHistoryId int64
type ProjectHistoryType string

type FlushedDoc struct {
	Lines              Lines              `json:"lines"`
	Version            Version            `json:"version"`
	Ranges             Ranges             `json:"ranges"`
	PathName           PathName           `json:"pathname"`
	ProjectHistoryId   ProjectHistoryId   `json:"projectHistoryId,omitempty"`
	ProjectHistoryType ProjectHistoryType `json:"projectHistoryType,omitempty"`
}

type PathName string

type DocCore struct {
	Snapshot         Snapshot           `json:"snapshot"`
	Hash             Hash               `json:"hash"`
	JsonRanges       json.RawMessage    `json:"json_ranges"`
	Ranges           Ranges             `json:"-"`
	ProjectId        primitive.ObjectID `json:"project_id"`
	PathName         PathName           `json:"path_name"`
	ProjectHistoryId ProjectHistoryId   `json:"project_history_id,omitempty"`
}

type Doc struct {
	DocCore
	LastUpdatedCtx
	Version
	UnFlushedTime
}

func (core *DocCore) UnmarshalJSON(bytes []byte) error {
	if err := json.Unmarshal(bytes, &core); err != nil {
		return err
	}
	hash := core.Snapshot.Hash()
	if core.Hash != hash {
		return errors.New(
			string("snapshot hash mismatch: " + core.Hash + " != " + hash),
		)
	}
	if err := json.Unmarshal(core.JsonRanges, &core.Ranges); err != nil {
		return err
	}
	return nil
}

func (core *DocCore) MarshalJSON() ([]byte, error) {
	ranges, err := json.Marshal(core.Ranges)
	if err != nil {
		return nil, err
	}
	if len(ranges) > MaxRangesSize {
		return nil, errors.New("ranges are too large")
	}
	core.JsonRanges = ranges

	core.Hash = core.Snapshot.Hash()

	return json.Marshal(core)
}
