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
	"strconv"

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
	Type             string             `json:"type"`
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

type Snippet []rune

func (s *Snippet) UnmarshalJSON(bytes []byte) error {
	var raw string
	if err := json.Unmarshal(bytes, &raw); err != nil {
		return err
	}
	*s = Snippet(raw)
	return nil
}

func (s Snippet) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}

type Component struct {
	Comment   Snippet             `json:"c,omitempty"`
	Deletion  Snippet             `json:"d,omitempty"`
	Insertion Snippet             `json:"i,omitempty"`
	Position  int                 `json:"p"`
	Thread    *primitive.ObjectID `json:"t,omitempty"`
	Undo      bool                `json:"undo,omitempty"`
}

func (o *Component) IsComment() bool {
	return len(o.Comment) != 0
}
func (o *Component) IsDeletion() bool {
	return len(o.Deletion) != 0
}
func (o *Component) IsInsertion() bool {
	return len(o.Insertion) != 0
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
	DupIfSource DupIfSource        `json:"dupIfSource,omitempty"`
	Hash        Hash               `json:"hash,omitempty"`
	Meta        DocumentUpdateMeta `json:"meta"`
	Op          Op                 `json:"op"`
	Version     Version            `json:"v"`
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
		a := strconv.FormatInt(int64(d.Version), 10)
		b := strconv.FormatInt(int64(current), 10)
		return &errors.ValidationError{
			Msg: "Op at future version: " + a + " vs " + b,
		}
	}
	if d.Version+maxAgeOfOp < current {
		a := strconv.FormatInt(int64(d.Version+maxAgeOfOp), 10)
		b := strconv.FormatInt(int64(current), 10)
		return &errors.ValidationError{
			Msg: "Op too old: " + a + " vs " + b,
		}
	}
	return nil
}

type AppliedOpsMessage struct {
	DocId       primitive.ObjectID      `json:"doc_id"`
	Error       *errors.JavaScriptError `json:"error,omitempty"`
	HealthCheck bool                    `json:"health_check,omitempty"`
	Update      DocumentUpdate          `json:"op,omitempty"`
}
