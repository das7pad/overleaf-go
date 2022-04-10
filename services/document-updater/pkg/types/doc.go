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

package types

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/doc"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type UnFlushedTime int64

type LastUpdatedCtx struct {
	At time.Time   `json:"at"`
	By edgedb.UUID `json:"by,omitempty"`
}

type DocCore struct {
	Snapshot  sharedTypes.Snapshot `json:"snapshot"`
	Hash      sharedTypes.Hash     `json:"hash"`
	ProjectId edgedb.UUID          `json:"project_id"`
	PathName  sharedTypes.PathName `json:"path_name"`
}

type Doc struct {
	DocCore
	LastUpdatedCtx
	sharedTypes.Version
	UnFlushedTime
	DocId               edgedb.UUID
	JustLoadedIntoRedis bool
}

func DocFromFlushedDoc(flushedDoc *project.Doc, projectId, docId edgedb.UUID) *Doc {
	d := &Doc{}
	d.DocId = docId
	d.JustLoadedIntoRedis = true
	d.PathName = flushedDoc.ResolvedPath
	d.ProjectId = projectId
	d.Snapshot = sharedTypes.Snapshot(flushedDoc.Snapshot)
	d.Version = flushedDoc.Version
	return d
}

type SetDocRequest struct {
	Lines    sharedTypes.Lines    `json:"lines"`
	Snapshot sharedTypes.Snapshot `json:"snapshot"`
	Source   string               `json:"source"`
	UserId   edgedb.UUID          `json:"user_id"`
	Undoing  bool                 `json:"undoing"`
}

func (s *SetDocRequest) GetSnapshot() sharedTypes.Snapshot {
	if s.Snapshot == nil {
		s.Snapshot = s.Lines.ToSnapshot()
	}
	return s.Snapshot
}

func (s *SetDocRequest) Validate() error {
	if err := s.GetSnapshot().Validate(); err != nil {
		return err
	}
	return nil
}

func (d *Doc) ToSetDocDetails() *doc.ForDocUpdate {
	return &doc.ForDocUpdate{
		Snapshot:      d.Snapshot,
		Version:       d.Version,
		LastUpdatedAt: d.LastUpdatedCtx.At,
		LastUpdatedBy: d.LastUpdatedCtx.By,
	}
}

type DocContentLines struct {
	Id       edgedb.UUID          `json:"_id"`
	Lines    sharedTypes.Lines    `json:"lines"`
	PathName sharedTypes.PathName `json:"pathname"`
	Version  sharedTypes.Version  `json:"v"`
}

type DocContentSnapshot struct {
	Id            edgedb.UUID          `json:"_id"`
	Snapshot      string               `json:"snapshot"`
	PathName      sharedTypes.PathName `json:"pathname"`
	Version       sharedTypes.Version  `json:"v"`
	LastUpdatedAt time.Time            `json:"-"`
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
		Id:            d.DocId,
		Snapshot:      string(d.Snapshot),
		PathName:      d.PathName,
		Version:       d.Version,
		LastUpdatedAt: d.LastUpdatedCtx.At,
	}
}

type DocContentSnapshots []*DocContentSnapshot

var unixEpochStart = time.Unix(0, 0)

func (l DocContentSnapshots) LastUpdatedAt() time.Time {
	if l == nil {
		return unixEpochStart
	}
	max := time.Time{}
	for _, snapshot := range l {
		if snapshot.LastUpdatedAt.After(max) {
			max = snapshot.LastUpdatedAt
		}
	}
	return max
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

	// parts[2] are Ranges

	core.ProjectId, err = edgedb.ParseUUID(string(parts[3]))
	if err != nil {
		return errors.Tag(err, "cannot parse projectId")
	}

	if err = json.Unmarshal(parts[4], &core.PathName); err != nil {
		return errors.Tag(err, "cannot parse pathName")
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
	return nil
}

func (core *DocCore) DoMarshalJSON() ([]byte, error) {
	core.Hash = core.Snapshot.Hash()
	return json.Marshal(core)
}
