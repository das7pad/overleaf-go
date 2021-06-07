// Copyright 2021 Jakob Ackermann <das7pad@outlook.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Change struct {
	RangeEntryBase
	Op ChangeOp `json:"op"`
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
	if c == nil {
		return other == nil
	}
	if other == nil {
		return false
	}
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
	Deletion  string           `json:"d,omitempty"`
	Insertion string           `json:"i,omitempty"`
	Position  JavaScriptNumber `json:"p"`
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
	RangeEntryBase
	Op CommentOp `json:"op"`
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
	if c == nil {
		return other == nil
	}
	if other == nil {
		return false
	}
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
	Comment  string             `json:"c"`
	Position JavaScriptNumber   `json:"p"`
	Thread   primitive.ObjectID `json:"t"`
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
	if l == nil {
		return other == nil
	}
	if other == nil {
		return false
	}
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
	Timestamp time.Time           `json:"ts"`
	UserId    *primitive.ObjectID `json:"user_id,omitempty"`
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
	Id       primitive.ObjectID `json:"id"`
	MetaData RangeMetaData      `json:"metadata"`
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
	Changes  Changes  `json:"changes,omitempty"`
	Comments Comments `json:"comments,omitempty"`
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
