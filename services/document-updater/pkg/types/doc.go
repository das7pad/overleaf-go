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
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"strconv"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/document-updater/pkg/errors"
)

const megabytes = 1024 * 1024
const MaxRangesSize = 3 * megabytes

type UnFlushedTime int64

func (u *UnFlushedTime) UnmarshalJSON(bytes []byte) error {
	if bytes == nil {
		return nil
	}
	raw, err := strconv.ParseInt(string(bytes), 10, 64)
	if err != nil {
		return err
	}
	*u = UnFlushedTime(raw)
	return nil
}

type LastUpdatedCtx struct {
	At int64              `json:"at"`
	By primitive.ObjectID `json:"by,omitempty"`
}

type ProjectHistoryId int64
type ProjectHistoryType string

type FlushedDoc struct {
	Lines              Lines              `json:"lines"`
	PathName           PathName           `json:"pathname"`
	ProjectHistoryId   ProjectHistoryId   `json:"projectHistoryId,omitempty"`
	ProjectHistoryType ProjectHistoryType `json:"projectHistoryType,omitempty"`
	Ranges             Ranges             `json:"ranges"`
	Version            Version            `json:"version"`
}

func (d *FlushedDoc) ToDoc(projectId primitive.ObjectID) *Doc {
	doc := &Doc{}
	doc.Snapshot = d.Lines.ToSnapshot()
	doc.PathName = d.PathName
	doc.ProjectHistoryId = d.ProjectHistoryId
	doc.ProjectId = projectId
	doc.Ranges = d.Ranges
	doc.Version = d.Version
	return doc
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

type SetDocDetails struct {
	Lines         Lines              `json:"lines"`
	Ranges        Ranges             `json:"ranges"`
	Version       Version            `json:"version"`
	LastUpdatedAt int64              `json:"lastUpdatedAt"`
	LastUpdatedBy primitive.ObjectID `json:"lastUpdatedBy"`
}

func (d *Doc) ToSetDocDetails() *SetDocDetails {
	return &SetDocDetails{
		Lines:         d.Snapshot.ToLines(),
		Ranges:        d.Ranges,
		Version:       d.Version,
		LastUpdatedAt: d.LastUpdatedCtx.At,
		LastUpdatedBy: d.LastUpdatedCtx.By,
	}
}

func deserializeDocCoreV0(core *DocCore, blob []byte) error {
	var err error
	parts := bytes.Split(blob, []byte{0})
	if len(parts) != 6 {
		n := strconv.FormatInt(int64(len(parts)), 10)
		return errors.New("expected 6 doc core parts, got " + n)
	}

	d := sha1.New()
	d.Write(parts[0])
	hash := Hash(hex.EncodeToString(d.Sum(nil)))

	if err = hash.CheckMatches(Hash(parts[1])); err != nil {
		return err
	}

	var lines Lines
	if err = json.Unmarshal(parts[0], &lines); err != nil {
		return errors.Tag(err, "cannot parse lines")
	}
	core.Snapshot = lines.ToSnapshot()

	core.JsonRanges = parts[2]

	core.ProjectId, err = primitive.ObjectIDFromHex(string(parts[3]))
	if err != nil {
		return errors.Tag(err, "cannot parse projectId")
	}

	if err = json.Unmarshal(parts[4], &core.PathName); err != nil {
		return errors.Tag(err, "cannot parse pathName")
	}

	if string(parts[5]) == "NaN" || string(parts[5]) == "undefined" {
		core.ProjectHistoryId = 0
	} else {
		if err = json.Unmarshal(parts[5], &core.ProjectHistoryId); err != nil {
			return errors.Tag(err, "cannot parse projectHistoryId")
		}
	}
	return nil
}

func (core *DocCore) DoUnmarshalJSON(bytes []byte) error {
	if len(bytes) == 0 {
		return errors.New("empty doc core blob")
	}
	if bytes[0] == '{' {
		if err := json.Unmarshal(bytes, &core); err != nil {
			return err
		}
		hash := core.Snapshot.Hash()
		if err := core.Hash.CheckMatches(hash); err != nil {
			return err
		}
	} else {
		if err := deserializeDocCoreV0(core, bytes); err != nil {
			return err
		}
	}
	if err := json.Unmarshal(core.JsonRanges, &core.Ranges); err != nil {
		return errors.Tag(err, "cannot deserialize ranges")
	}
	return nil
}

func (core *DocCore) DoMarshalJSON() ([]byte, error) {
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
