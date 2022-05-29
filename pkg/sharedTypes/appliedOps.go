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

package sharedTypes

import (
	"strconv"
	"time"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/errors"
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

func (t *Timestamp) ParseIfSet(s string) error {
	if s == "" {
		return nil
	}
	raw, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return &errors.ValidationError{Msg: "invalid timestamp (int)"}
	}
	*t = Timestamp(raw)
	return nil
}

func (t Timestamp) ToTime() time.Time {
	ms := int64(t)
	return time.Unix(ms/1000, ms%1000*int64(time.Millisecond))
}

type DocumentUpdateMeta struct {
	Type             string           `json:"type,omitempty"`
	Source           PublicId         `json:"source"`
	Timestamp        Timestamp        `json:"ts,omitempty"`
	TrackChangesSeed TrackChangesSeed `json:"tc,omitempty"`
	UserId           edgedb.UUID      `json:"user_id,omitempty"`
	IngestionTime    *time.Time       `json:"ingestion_time,omitempty"`
}

func (d *DocumentUpdateMeta) Validate() error {
	if err := d.Source.Validate(); err != nil {
		return err
	}
	if err := d.TrackChangesSeed.Validate(); err != nil {
		return err
	}
	return nil
}

type Component struct {
	Deletion  Snippet `json:"d,omitempty"`
	Insertion Snippet `json:"i,omitempty"`
	Position  int     `json:"p"`
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
	if o.IsDeletion() {
		return nil
	} else if o.IsInsertion() {
		return Snapshot(o.Insertion).Validate()
	} else {
		return &errors.ValidationError{Msg: "unknown op type"}
	}
}

type Op []Component

func (o Op) Validate() error {
	if len(o) == 0 {
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
	DocId       edgedb.UUID        `json:"doc"`
	Dup         bool               `json:"dup,omitempty"`
	DupIfSource DupIfSource        `json:"dupIfSource,omitempty"`
	Hash        Hash               `json:"hash,omitempty"`
	Meta        DocumentUpdateMeta `json:"meta"`
	Op          Op                 `json:"op"`
	Version     Version            `json:"v"`
}

func (d *DocumentUpdate) Validate() error {
	if d.Dup {
		if len(d.Op) != 0 {
			return &errors.ValidationError{
				Msg: "non empty op on duplicate update",
			}
		}
	} else {
		if err := d.Op.Validate(); err != nil {
			return err
		}
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
		a := d.Version.String()
		b := current.String()
		return &errors.ValidationError{
			Msg: "Op at future version: " + a + " vs " + b,
		}
	}
	if d.Version+maxAgeOfOp < current {
		a := (d.Version + maxAgeOfOp).String()
		b := current.String()
		return &errors.ValidationError{
			Msg: "Op too old: " + a + " vs " + b,
		}
	}
	return nil
}

type DocumentUpdateAck struct {
	DocId   edgedb.UUID `json:"doc"`
	Version Version     `json:"v"`
}

type AppliedOpsMessage struct {
	DocId       edgedb.UUID             `json:"doc_id"`
	Error       *errors.JavaScriptError `json:"error,omitempty"`
	HealthCheck bool                    `json:"health_check,omitempty"`
	Update      *DocumentUpdate         `json:"op,omitempty"`
	ProcessedBy string                  `json:"processed_by,omitempty"`
}

func (m *AppliedOpsMessage) ChannelId() edgedb.UUID {
	return m.DocId
}

func (m *AppliedOpsMessage) Validate() error {
	if m.Update != nil {
		if err := m.Update.Validate(); err != nil {
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
