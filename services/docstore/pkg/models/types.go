// Golang port of the Overleaf docstore service
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

package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Change struct {
	RangeEntryBase `bson:"inline"`
	Op             ChangeOp `json:"op" bson:"op"`
}

func (c Change) Equals(other Change) bool {
	if !c.RangeEntryBase.Equals(other.RangeEntryBase) {
		return false
	}
	if !c.Op.Equals(other.Op) {
		return false
	}
	return true
}

type Changes []Change

func (c Changes) Equals(other Changes) bool {
	if len(c) != len(other) {
		return false
	}
	for i, change := range c {
		if !change.Equals(other[i]) {
			return false
		}
	}
	return true
}

type ChangeOp struct {
	Deletion  string           `json:"d,omitempty" bson:"d,omitempty"`
	Insertion string           `json:"i,omitempty" bson:"i,omitempty"`
	Position  JavaScriptNumber `json:"p" bson:"p"`
}

func (o ChangeOp) Equals(other ChangeOp) bool {
	if o.Deletion != other.Deletion {
		return false
	}
	if o.Insertion != other.Insertion {
		return false
	}
	if o.Position != other.Position {
		return false
	}
	return true
}

type Comment struct {
	RangeEntryBase `bson:"inline"`
	Op             CommentOp `json:"op" bson:"op"`
}

func (c Comment) Equals(other Comment) bool {
	if !c.RangeEntryBase.Equals(other.RangeEntryBase) {
		return false
	}
	if !c.Op.Equals(other.Op) {
		return false
	}
	return true
}

type Comments []Comment

func (c Comments) Equals(other Comments) bool {
	if len(c) != len(other) {
		return false
	}
	for i, change := range c {
		if !change.Equals(other[i]) {
			return false
		}
	}
	return true
}

type CommentOp struct {
	Comment  string             `json:"c" bson:"c"`
	Position JavaScriptNumber   `json:"p" bson:"p"`
	Thread   primitive.ObjectID `json:"t" bson:"t"`
}

func (o CommentOp) Equals(other CommentOp) bool {
	if o.Comment != other.Comment {
		return false
	}
	if o.Position != other.Position {
		return false
	}
	if o.Thread.Hex() != other.Thread.Hex() {
		return false
	}
	return true
}

type JavaScriptNumber float64

type Lines []string

func (l Lines) Equals(other Lines) bool {
	if len(l) != len(other) {
		return false
	}
	for i, line := range l {
		if line != other[i] {
			return false
		}
	}
	return true
}

type RangeMetaData struct {
	Timestamp time.Time           `json:"ts" bson:"ts"`
	UserId    *primitive.ObjectID `json:"user_id,omitempty" bson:"user_id,omitempty"`
}

func (d RangeMetaData) Equals(other RangeMetaData) bool {
	if !d.Timestamp.Equal(other.Timestamp) {
		return false
	}
	if d.UserId == nil && other.UserId != nil {
		return false
	}
	if other.UserId == nil && d.UserId != nil {
		return false
	}
	if d.UserId.Hex() != other.UserId.Hex() {
		return false
	}
	return true
}

type RangeEntryBase struct {
	Id       primitive.ObjectID `json:"id" bson:"id"`
	MetaData RangeMetaData      `json:"metadata" bson:"metadata"`
}

func (b RangeEntryBase) Equals(other RangeEntryBase) bool {
	if b.Id != other.Id {
		return false
	}
	if !b.MetaData.Equals(other.MetaData) {
		return false
	}
	return true
}

type Ranges struct {
	Changes  Changes  `json:"changes,omitempty" bson:"changes,omitempty"`
	Comments Comments `json:"comments,omitempty" bson:"comments,omitempty"`
}

func (r Ranges) Equals(other Ranges) bool {
	if !r.Changes.Equals(other.Changes) {
		return false
	}
	if !r.Comments.Equals(other.Comments) {
		return false
	}
	return true
}

type Revision JavaScriptNumber

type Version JavaScriptNumber

func (v Version) Equals(other Version) bool {
	return v == other
}
