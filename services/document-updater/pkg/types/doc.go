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

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

const megabytes = 1024 * 1024
const MaxRangesSize = 3 * megabytes

type UnFlushedTime int64

type LastUpdatedCtx struct {
	At int64              `json:"at"`
	By primitive.ObjectID `json:"by,omitempty"`
}

type ProjectHistoryId int64
type ProjectHistoryType string

type FlushedDoc struct {
	Lines              sharedTypes.Lines    `json:"lines"`
	PathName           sharedTypes.PathName `json:"pathname"`
	ProjectHistoryId   ProjectHistoryId     `json:"projectHistoryId,omitempty"`
	ProjectHistoryType ProjectHistoryType   `json:"projectHistoryType,omitempty"`
	Ranges             sharedTypes.Ranges   `json:"ranges"`
	Version            sharedTypes.Version  `json:"version"`
}

func (d *FlushedDoc) ToDoc(projectId, docId primitive.ObjectID) *Doc {
	doc := &Doc{}
	doc.DocId = docId
	doc.Snapshot = d.Lines.ToSnapshot()
	doc.PathName = d.PathName
	doc.ProjectHistoryId = d.ProjectHistoryId
	doc.ProjectId = projectId
	doc.Ranges = d.Ranges
	doc.Version = d.Version
	doc.JustLoadedIntoRedis = true
	return doc
}

type DocCore struct {
	Snapshot         sharedTypes.Snapshot `json:"snapshot"`
	Hash             sharedTypes.Hash     `json:"hash"`
	JsonRanges       json.RawMessage      `json:"json_ranges"`
	Ranges           sharedTypes.Ranges   `json:"-"`
	ProjectId        primitive.ObjectID   `json:"project_id"`
	PathName         sharedTypes.PathName `json:"path_name"`
	ProjectHistoryId ProjectHistoryId     `json:"project_history_id,omitempty"`
}

type Doc struct {
	DocCore
	LastUpdatedCtx
	sharedTypes.Version
	UnFlushedTime
	DocId               primitive.ObjectID
	JustLoadedIntoRedis bool
}

type SetDocRequest struct {
	Lines    sharedTypes.Lines `json:"lines"`
	snapshot sharedTypes.Snapshot
	Source   string             `json:"source"`
	UserId   primitive.ObjectID `json:"user_id"`
	Undoing  bool               `json:"undoing"`
}

func (s *SetDocRequest) Snapshot() sharedTypes.Snapshot {
	if s.snapshot == nil {
		s.snapshot = s.Lines.ToSnapshot()
	}
	return s.snapshot
}

func (s *SetDocRequest) Validate() error {
	if err := s.Snapshot().Validate(); err != nil {
		return err
	}
	return nil
}

type SetDocDetails struct {
	Lines         sharedTypes.Lines   `json:"lines"`
	Ranges        sharedTypes.Ranges  `json:"ranges"`
	Version       sharedTypes.Version `json:"version"`
	LastUpdatedAt int64               `json:"lastUpdatedAt"`
	LastUpdatedBy primitive.ObjectID  `json:"lastUpdatedBy"`
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

type DocContentLines struct {
	Id       primitive.ObjectID   `json:"_id"`
	Lines    sharedTypes.Lines    `json:"lines"`
	PathName sharedTypes.PathName `json:"pathname"`
	Version  sharedTypes.Version  `json:"v"`
}

type DocContentSnapshot struct {
	Id       primitive.ObjectID   `json:"_id"`
	Snapshot sharedTypes.Snapshot `json:"snapshot"`
	PathName sharedTypes.PathName `json:"pathname"`
	Version  sharedTypes.Version  `json:"v"`
}

func (d *Doc) ToDocContentLines() *DocContentLines {
	return &DocContentLines{
		Id:       d.DocId,
		Lines:    d.Snapshot.ToLines(),
		PathName: d.PathName,
		Version:  d.Version,
	}
}

func (d *Doc) ToDocContentSnapshot() *DocContentSnapshot {
	return &DocContentSnapshot{
		Id:       d.DocId,
		Snapshot: d.Snapshot,
		PathName: d.PathName,
		Version:  d.Version,
	}
}

func deserializeDocCoreV0(core *DocCore, blob []byte) error {
	var err error
	parts := bytes.Split(blob, []byte{0})
	if len(parts) != 6 {
		n := sharedTypes.Int(len(parts)).String()
		return errors.New("expected 6 doc core parts, got " + n)
	}

	d := sha1.New()
	d.Write(parts[0])
	hash := sharedTypes.Hash(hex.EncodeToString(d.Sum(nil)))

	if err = hash.CheckMatches(sharedTypes.Hash(parts[1])); err != nil {
		return err
	}

	var lines sharedTypes.Lines
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
