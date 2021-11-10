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

package project

import (
	"fmt"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type TreeElement interface {
	GetId() primitive.ObjectID
	GetName() sharedTypes.Filename
}

type CommonTreeFields struct {
	Id   primitive.ObjectID   `json:"_id" bson:"_id"`
	Name sharedTypes.Filename `json:"name" bson:"name"`
}

func (c CommonTreeFields) GetId() primitive.ObjectID {
	return c.Id
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

	LinkedFileData *LinkedFileData  `json:"linkedFileData,omitempty" bson:"linkedFileData,omitempty"`
	Hash           sharedTypes.Hash `json:"hash" bson:"hash"`
	Created        time.Time        `json:"created" bson:"created"`
}

type Folder struct {
	CommonTreeFields `bson:"inline"`

	Docs     []Doc     `json:"docs" bson:"docs"`
	FileRefs []FileRef `json:"fileRefs" bson:"fileRefs"`
	Folders  []Folder  `json:"folders" bson:"folders"`
}

var AbortWalk = errors.New("abort walk")

type MongoPath string

type DirWalker func(folder *Folder, path sharedTypes.DirName) error
type DirWalkerMongo func(folder *Folder, path sharedTypes.DirName, mongoPath MongoPath) error
type TreeWalker func(element TreeElement, path sharedTypes.PathName) error

var (
	ErrDuplicateNameInFolder = &errors.InvalidStateError{
		Msg: "folder already has entry with given name",
	}
)

func (t *Folder) CheckIsUniqueName(needle sharedTypes.Filename) error {
	if t.HasEntry(needle) {
		return ErrDuplicateNameInFolder
	}
	return nil
}

func (t *Folder) HasEntry(needle sharedTypes.Filename) bool {
	for _, doc := range t.Docs {
		if doc.Name == needle {
			return true
		}
	}
	for _, file := range t.FileRefs {
		if file.Name == needle {
			return true
		}
	}
	for _, folder := range t.Folders {
		if folder.Name == needle {
			return true
		}
	}
	return false
}

func (t *Folder) Walk(fn TreeWalker) error {
	return ignoreAbort(t.walk(fn, "", walkModeAny))
}

func (t *Folder) WalkDocs(fn TreeWalker) error {
	return ignoreAbort(t.walk(fn, "", walkModeDoc))
}

func (t *Folder) WalkFiles(fn TreeWalker) error {
	return ignoreAbort(t.walk(fn, "", walkModeFiles))
}

func (t *Folder) WalkFolders(fn DirWalker) error {
	return ignoreAbort(t.walkDirs(fn, ""))
}

func (t *Folder) WalkFoldersMongo(fn DirWalkerMongo) error {
	return ignoreAbort(t.walkDirsMongo(fn, "", "rootFolder.0"))
}

type walkMode int

const (
	walkModeDoc walkMode = iota
	walkModeFiles
	walkModeAny
)

func ignoreAbort(err error) error {
	if err != nil && err != AbortWalk {
		return err
	}
	return nil
}

func (t *Folder) walkIgnoreAbort(fn TreeWalker, m walkMode) error {
	err := t.walk(fn, "", m)
	if err != nil && err != AbortWalk {
		return err
	}
	return nil
}

func (t *Folder) walkDirs(fn DirWalker, parent sharedTypes.DirName) error {
	if err := fn(t, parent); err != nil {
		return err
	}
	for _, folder := range t.Folders {
		branch := sharedTypes.DirName(parent.Join(folder.Name))
		if err := folder.walkDirs(fn, branch); err != nil {
			return err
		}
	}
	return nil
}

func (t *Folder) walkDirsMongo(fn DirWalkerMongo, parent sharedTypes.DirName, mongoParent MongoPath) error {
	if err := fn(t, parent, mongoParent); err != nil {
		return err
	}
	mongoParent += ".folders."
	for i, folder := range t.Folders {
		branch := sharedTypes.DirName(parent.Join(folder.Name))
		s := MongoPath(strconv.FormatInt(int64(i), 10))
		if err := folder.walkDirsMongo(fn, branch, mongoParent+s); err != nil {
			return err
		}
	}
	return nil
}

func (t *Folder) walk(fn TreeWalker, parent sharedTypes.DirName, m walkMode) error {
	if m == walkModeDoc || m == walkModeAny {
		for _, doc := range t.Docs {
			if err := fn(doc, parent.Join(doc.Name)); err != nil {
				return err
			}
		}
	}
	if m == walkModeFiles || m == walkModeAny {
		for _, fileRef := range t.FileRefs {
			if err := fn(fileRef, parent.Join(fileRef.Name)); err != nil {
				return err
			}
		}
	}
	for _, folder := range t.Folders {
		branch := sharedTypes.DirName(parent.Join(folder.Name))
		if err := folder.walk(fn, branch, m); err != nil {
			return err
		}
	}
	return nil
}

func (p *TreeField) GetRootFolder() (*Folder, error) {
	if len(p.RootFolder) != 1 {
		return nil, &errors.ValidationError{
			Msg: fmt.Sprintf(
				"expected rootFolder to have 1 entry, got %d",
				len(p.RootFolder),
			),
		}
	}
	return &p.RootFolder[0], nil
}
