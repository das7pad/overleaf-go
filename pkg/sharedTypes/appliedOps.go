// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type PublicId string

func (p PublicId) Validate() error {
	if p == "" {
		return &errors.ValidationError{Msg: "missing client id"}
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

type AppliedDocumentUpdateMeta struct {
	Source PublicId `json:"source"`
	Type   string   `json:"type,omitempty"`
}

type DocumentUpdateMeta struct {
	IngestionTime time.Time `json:"ingestion_time,omitempty"`
	Source        PublicId  `json:"source"`
	Type          string    `json:"type,omitempty"`
	Timestamp     Timestamp `json:"ts,omitempty"`
	UserId        UUID      `json:"user_id,omitempty"`
}

func (d *DocumentUpdateMeta) Validate() error {
	if err := d.Source.Validate(); err != nil {
		return errors.Tag(err, "source")
	}
	return nil
}

type Component struct {
	Deletion  Snippet `json:"d,omitempty"`
	Insertion Snippet `json:"i,omitempty"`
	Position  int     `json:"p"`
}

func (c Component) IsDeletion() bool {
	return len(c.Deletion) != 0
}

func (c Component) IsInsertion() bool {
	return len(c.Insertion) != 0
}

func (c Component) IsNoOp() bool {
	return len(c.Deletion) == 0 && len(c.Insertion) == 0
}

func (c Component) Validate() error {
	if c.Position < 0 {
		return &errors.ValidationError{Msg: "position is negative"}
	}
	switch {
	case c.IsDeletion():
		return nil
	case c.IsInsertion():
		return Snapshot(c.Insertion).Validate()
	default:
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

type AppliedDocumentUpdate struct {
	DocId   UUID                      `json:"doc"`
	Meta    AppliedDocumentUpdateMeta `json:"meta"`
	Op      Op                        `json:"op"`
	Version Version                   `json:"v"`
}

type DocumentUpdate struct {
	DocId       UUID               `json:"doc"`
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
	DocId   UUID    `json:"doc"`
	Version Version `json:"v"`
}

func (d *DocumentUpdateAck) MarshalJSON() ([]byte, error) {
	buf := make([]byte, 0, len(`{"doc":"","v":}`)+20+36)
	buf = append(buf, `{"doc":"`...)
	buf = d.DocId.Append(buf)
	buf = append(buf, `","v":`...)
	buf = strconv.AppendUint(buf, uint64(d.Version), 10)
	buf = append(buf, `}`...)
	return buf, nil
}

type AppliedOpsErrorMeta struct {
	Error errors.JavaScriptError `json:"error"`
	DocId UUID                   `json:"docId"`
}
