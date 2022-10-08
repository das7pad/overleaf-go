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
	"encoding/json"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/doc"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type UnFlushedTime int64

type LastUpdatedCtx struct {
	At time.Time        `json:"at"`
	By sharedTypes.UUID `json:"by,omitempty"`
}

type DocCore struct {
	Snapshot  sharedTypes.Snapshot `json:"snapshot"`
	Hash      sharedTypes.Hash     `json:"hash"`
	ProjectId sharedTypes.UUID     `json:"project_id"`
	PathName  sharedTypes.PathName `json:"path_name"`
}

type Doc struct {
	DocCore
	LastUpdatedCtx
	sharedTypes.Version
	UnFlushedTime
	DocId               sharedTypes.UUID
	JustLoadedIntoRedis bool
}

func DocFromFlushedDoc(flushedDoc *project.Doc, projectId, docId sharedTypes.UUID) *Doc {
	d := &Doc{}
	d.DocId = docId
	d.JustLoadedIntoRedis = true
	d.PathName = flushedDoc.Path
	d.ProjectId = projectId
	d.Snapshot = sharedTypes.Snapshot(flushedDoc.Snapshot)
	d.Version = flushedDoc.Version
	return d
}

type SetDocRequest struct {
	Snapshot sharedTypes.Snapshot `json:"snapshot"`
	Source   string               `json:"source"`
	UserId   sharedTypes.UUID     `json:"user_id"`
	Undoing  bool                 `json:"undoing"`
}

func (s *SetDocRequest) Validate() error {
	if err := s.Snapshot.Validate(); err != nil {
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

type DocContentSnapshot struct {
	Id            sharedTypes.UUID     `json:"_id"`
	Snapshot      string               `json:"snapshot"`
	PathName      sharedTypes.PathName `json:"pathname"`
	Version       sharedTypes.Version  `json:"v"`
	LastUpdatedAt time.Time            `json:"-"`
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

func (l DocContentSnapshots) LastUpdatedAt() time.Time {
	max := time.Time{}
	for _, snapshot := range l {
		if snapshot.LastUpdatedAt.After(max) {
			max = snapshot.LastUpdatedAt
		}
	}
	return max
}

func (core *DocCore) DoUnmarshalJSON(bytes []byte) error {
	if len(bytes) == 0 {
		return errors.New("empty doc core blob")
	}
	if err := json.Unmarshal(bytes, &core); err != nil {
		return err
	}
	hash := core.Snapshot.Hash()
	if err := core.Hash.CheckMatches(hash); err != nil {
		return err
	}
	return nil
}

func (core *DocCore) DoMarshalJSON() ([]byte, error) {
	core.Hash = core.Snapshot.Hash()
	return json.Marshal(core)
}
