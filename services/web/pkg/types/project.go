// Golang port of Overleaf
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
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type TreeElement interface {
	GetName() sharedTypes.Filename
}

type CommonTreeFields struct {
	Id   primitive.ObjectID   `json:"_id" bson:"_id"`
	Name sharedTypes.Filename `json:"name" bson:"name"`
}

func (c CommonTreeFields) GetName() sharedTypes.Filename {
	return c.Name
}

type Doc struct {
	CommonTreeFields `bson:"inline"`
}

type LinkedFileData struct {
	Provider         string `json:"provider" bson:"provider"`
	SourceProjectId  string `json:"source_project_id" bson:"source_project_id"`
	SourceEntityPath string `json:"source_entity_path" bson:"source_entity_path"`
}

type FileRef struct {
	CommonTreeFields `bson:"inline"`

	LinkedFileData *LinkedFileData  `json:"linked_file_data,omitempty" bson:"linked_file_data,omitempty"`
	Hash           sharedTypes.Hash `json:"hash" bson:"hash"`
	Created        time.Time        `json:"created" bson:"created"`
}

type Folder struct {
	CommonTreeFields `bson:"inline"`

	Docs     []Doc     `bson:"docs"`
	FileRefs []FileRef `bson:"fileRefs"`
	Folders  []Folder
}

var AbortWalk = errors.New("abort walk")

type TreeWalker func(element TreeElement, path sharedTypes.PathName) error

func (t *Folder) Walk(fn TreeWalker) error {
	err := t.walk(fn, "")
	if err != nil && err != AbortWalk {
		return err
	}
	return nil
}

func (t *Folder) walk(fn TreeWalker, parent sharedTypes.DirName) error {
	for _, doc := range t.Docs {
		if err := fn(doc, parent.Join(doc.Name)); err != nil {
			return err
		}
	}
	for _, fileRef := range t.FileRefs {
		err := fn(fileRef, parent.Join(fileRef.Name))
		if err != nil {
			return err
		}
	}
	for _, folder := range t.Folders {
		err := folder.walk(fn, sharedTypes.DirName(parent.Join(folder.Name)))
		if err != nil {
			return err
		}
	}
	return nil
}
