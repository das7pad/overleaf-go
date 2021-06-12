// Golang port of the Overleaf real-time service
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
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/real-time/pkg/errors"
)

// generatePublicId yields a secure unique id
// It contains a 16 hex char long timestamp in ns precision, a hyphen and
//  another 16 hex char long random string.
func generatePublicId() (PublicId, error) {
	buf := make([]byte, 8)
	_, err := rand.Read(buf)
	if err != nil {
		return "", err
	}
	now := time.Now().UnixNano()
	id := PublicId(
		strconv.FormatInt(now, 16) + "-" + hex.EncodeToString(buf),
	)
	return id, nil
}

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
	UserId           primitive.ObjectID `json:"user_id"`
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

type Op struct {
	Comment   string              `json:"c,omitempty"`
	Deletion  string              `json:"d,omitempty"`
	Insertion string              `json:"i,omitempty"`
	Position  int64               `json:"p"`
	Thread    *primitive.ObjectID `json:"t,omitempty"`
}

func (o *Op) IsComment() bool {
	return o.Comment != ""
}
func (o *Op) IsDeletion() bool {
	return o.Deletion != ""
}
func (o *Op) IsInsertion() bool {
	return o.Insertion != ""
}
func (o *Op) Validate() error {
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

type Ops []Op

func (o Ops) HasEditOp() bool {
	for _, op := range o {
		if !op.IsComment() {
			return true
		}
	}
	return false
}

func (o Ops) HasCommentOp() bool {
	for _, op := range o {
		if op.IsComment() {
			return true
		}
	}
	return false
}

func (o Ops) Validate() error {
	if o == nil || len(o) == 0 {
		return &errors.ValidationError{Msg: "missing ops"}
	}
	for _, op := range o {
		if err := op.Validate(); err != nil {
			return err
		}
	}
	return nil
}

type DocumentUpdate struct {
	DocId       primitive.ObjectID `json:"doc"`
	Dup         bool               `json:"dup,omitempty"`
	DupIfSource []PublicId         `json:"dupIfSource,omitempty"`
	Hash        string             `json:"hash,omitempty"`
	Meta        DocumentUpdateMeta `json:"meta"`
	Ops         Ops                `json:"op"`
	Version     int64              `json:"v"`
	LastVersion int64              `json:"lastV"`
}
type MinimalDocumentUpdate struct {
	DocId   primitive.ObjectID `json:"doc"`
	Version int64              `json:"v"`
}

func (d *DocumentUpdate) Validate() error {
	if err := d.Ops.Validate(); err != nil {
		return err
	}
	if err := d.Meta.Validate(); err != nil {
		return err
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
