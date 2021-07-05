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
	"encoding/json"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/document-updater/pkg/errors"
)

type PublicId string

func (p PublicId) Validate() error {
	if p == "" {
		return &errors.ValidationError{Msg: "missing source"}
	}
	return nil
}

type TrackChangesSeed string

func (t TrackChangesSeed) Validate() error {
	if t == "" {
		return nil
	}
	if len(t) != 18 {
		return &errors.ValidationError{Msg: "tc must be 18 char long if set"}
	}
	return nil
}

type Timestamp int64

func (t Timestamp) Validate() error {
	if t == 0 {
		return nil
	}
	if t < 0 {
		return &errors.ValidationError{Msg: "ts must be greater zero"}
	}
	return nil
}

type DocumentUpdateMeta struct {
	Source           PublicId           `json:"source"`
	Timestamp        Timestamp          `json:"ts,omitempty"`
	TrackChangesSeed TrackChangesSeed   `json:"tc,omitempty"`
	UserId           primitive.ObjectID `json:"user_id,omitempty"`
}

func (d *DocumentUpdateMeta) Validate() error {
	if err := d.Source.Validate(); err != nil {
		return err
	}
	if err := d.Timestamp.Validate(); err != nil {
		return err
	}
	if err := d.TrackChangesSeed.Validate(); err != nil {
		return err
	}
	return nil
}

type Component struct {
	Comment   string              `json:"c,omitempty"`
	Deletion  string              `json:"d,omitempty"`
	Insertion string              `json:"i,omitempty"`
	Position  int64               `json:"p"`
	Thread    *primitive.ObjectID `json:"t,omitempty"`
	Undo      bool                `json:"undo,omitempty"`
}

func (o *Component) IsComment() bool {
	return o.Comment != ""
}
func (o *Component) IsDeletion() bool {
	return o.Deletion != ""
}
func (o *Component) IsInsertion() bool {
	return o.Insertion != ""
}
func (o *Component) Validate() error {
	if o.Position < 0 {
		return &errors.ValidationError{Msg: "position is negative"}
	}
	if o.IsComment() {
		if o.Thread == nil || o.Thread.IsZero() {
			return &errors.ValidationError{Msg: "comment op is missing thread"}
		}
		return nil
	} else if o.IsDeletion() {
		return nil
	} else if o.IsInsertion() {
		return nil
	} else {
		return &errors.ValidationError{Msg: "unknown op type"}
	}
}

type Op []Component

func (o Op) HasEdit() bool {
	for _, op := range o {
		if !op.IsComment() {
			return true
		}
	}
	return false
}

func (o Op) HasComment() bool {
	for _, op := range o {
		if op.IsComment() {
			return true
		}
	}
	return false
}

func (o Op) Validate() error {
	if o == nil || len(o) == 0 {
		return &errors.ValidationError{Msg: "missing ops"}
	}
	for _, component := range o {
		if err := component.Validate(); err != nil {
			return err
		}
	}
	return nil
}

type DupIfSource []PublicId

func (d DupIfSource) Contains(id PublicId) bool {
	for _, publicId := range d {
		if publicId == id {
			return true
		}
	}
	return false
}

type DocumentUpdate struct {
	DocId       primitive.ObjectID `json:"doc"`
	Dup         bool               `json:"dup,omitempty"`
	DupIfSource `json:"dupIfSource,omitempty"`
	Hash        string             `json:"hash,omitempty"`
	Meta        DocumentUpdateMeta `json:"meta"`
	Op          Op                 `json:"op"`
	Version     Version            `json:"v"`
	LastVersion int64              `json:"lastV"`
}

func (d *DocumentUpdate) Validate() error {
	if err := d.Op.Validate(); err != nil {
		return err
	}
	if err := d.Meta.Validate(); err != nil {
		return err
	}
	return nil
}

const maxAgeOfOp = Version(80)

func (d *DocumentUpdate) CheckVersion(current Version) error {
	if d.Version < 0 {
		return &errors.ValidationError{Msg: "Version missing"}
	}
	if d.Version > current {
		return &errors.ValidationError{Msg: "Op at future version"}
	}
	if d.Version+maxAgeOfOp < current {
		return &errors.ValidationError{Msg: "Op too old"}
	}
	return nil
}

type AppliedOpsMessage struct {
	DocId       primitive.ObjectID      `json:"doc_id"`
	Error       *errors.JavaScriptError `json:"error,omitempty"`
	HealthCheck bool                    `json:"health_check,omitempty"`
	UpdateRaw   json.RawMessage         `json:"op,omitempty"`
	update      *DocumentUpdate
}

func (m *AppliedOpsMessage) Update() (*DocumentUpdate, error) {
	if m.update != nil {
		return m.update, nil
	}
	d := json.NewDecoder(bytes.NewReader(m.UpdateRaw))
	d.DisallowUnknownFields()
	if err := d.Decode(&m.update); err != nil {
		return nil, err
	}
	return m.update, nil
}

func (m *AppliedOpsMessage) Validate() error {
	if m.UpdateRaw != nil {
		update, err := m.Update()
		if err != nil {
			return err
		}
		if err = update.Validate(); err != nil {
			return err
		}
		return nil
	} else if m.Error != nil {
		return nil
	} else if m.HealthCheck {
		return nil
	} else {
		return &errors.ValidationError{Msg: "unknown message type"}
	}
}
