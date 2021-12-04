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
	SetName(name sharedTypes.Filename)
	FieldNameInFolder() MongoPath
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

func (c *CommonTreeFields) SetName(name sharedTypes.Filename) {
	c.Name = name
}

type Doc struct {
	CommonTreeFields `bson:"inline"`
}

func (d *Doc) FieldNameInFolder() MongoPath {
	return "docs"
}

func NewDoc(name sharedTypes.Filename) *Doc {
	return &Doc{CommonTreeFields: CommonTreeFields{
		Id:   primitive.NewObjectID(),
		Name: name,
	}}
}

type LinkedFileProvider string

const (
	LinkedFileProviderURL               = "url"
	LinkedFileProviderProjectFile       = "project_file"
	LinkedFileProviderProjectOutputFile = "project_output_file"
)

func (p LinkedFileProvider) Validate() error {
	switch p {
	case LinkedFileProviderURL:
	case LinkedFileProviderProjectFile:
	case LinkedFileProviderProjectOutputFile:
	default:
		return &errors.ValidationError{Msg: "unknown provider"}
	}
	return nil
}

type LinkedFileData struct {
	Provider             LinkedFileProvider `json:"provider" bson:"provider"`
	SourceProjectId      string             `json:"source_project_id,omitempty" bson:"source_project_id,omitempty"`
	SourceEntityPath     string             `json:"source_entity_path,omitempty" bson:"source_entity_path,omitempty"`
	SourceOutputFilePath string             `json:"source_output_file_path,omitempty" bson:"source_output_file_path,omitempty"`
	URL                  *sharedTypes.URL   `json:"url,omitempty" bson:"url,omitempty"`
}

type FileRef struct {
	CommonTreeFields `bson:"inline"`

	LinkedFileData *LinkedFileData  `json:"linkedFileData,omitempty" bson:"linkedFileData,omitempty"`
	Hash           sharedTypes.Hash `json:"hash" bson:"hash"`
	Created        time.Time        `json:"created" bson:"created"`
	Size           *int64           `json:"size" bson:"size"`
}

func (f *FileRef) FieldNameInFolder() MongoPath {
	return "fileRefs"
}

func NewFileRef(name sharedTypes.Filename, hash sharedTypes.Hash, size *int64) *FileRef {
	return &FileRef{
		CommonTreeFields: CommonTreeFields{
			Id:   primitive.NewObjectID(),
			Name: name,
		},
		Created: time.Now().UTC(),
		Hash:    hash,
		Size:    size,
	}
}

type Folder struct {
	CommonTreeFields `bson:"inline"`

	Docs     []*Doc     `json:"docs" bson:"docs"`
	FileRefs []*FileRef `json:"fileRefs" bson:"fileRefs"`
	Folders  []*Folder  `json:"folders" bson:"folders"`
}

func (t *Folder) FieldNameInFolder() MongoPath {
	return "folders"
}

func (t *Folder) CreateParents(path sharedTypes.DirName) (*Folder, error) {
	if path == "." || path == "" {
		return t, nil
	}
	dir := path.Dir()
	name := path.Filename()
	parent, err := t.CreateParents(dir)
	if err != nil {
		return nil, err
	}
	entry, _ := parent.GetEntry(name)
	if entry == nil {
		folder := NewFolder(name)
		parent.Folders = append(parent.Folders, folder)
		return folder, nil
	}
	if folder, ok := entry.(*Folder); ok {
		return folder, nil
	}
	return nil, &errors.ValidationError{Msg: "conflicting paths"}
}

var ErrDuplicateFolderEntries = &errors.InvalidStateError{
	Msg: "duplicate folder entries",
}

func (t *Folder) CheckHasUniqueEntries() error {
	n := len(t.Docs) + len(t.FileRefs) + len(t.Folders)
	if n == 0 {
		return nil
	}
	names := make(map[sharedTypes.Filename]struct{}, n)
	for _, d := range t.Docs {
		if _, exists := names[d.Name]; exists {
			return ErrDuplicateFolderEntries
		}
		names[d.Name] = struct{}{}
	}
	for _, f := range t.FileRefs {
		if _, exists := names[f.Name]; exists {
			return ErrDuplicateFolderEntries
		}
		names[f.Name] = struct{}{}
	}
	for _, f := range t.Folders {
		if _, exists := names[f.Name]; exists {
			return ErrDuplicateFolderEntries
		}
		names[f.Name] = struct{}{}
	}
	return nil
}

func NewFolder(name sharedTypes.Filename) *Folder {
	return &Folder{
		CommonTreeFields: CommonTreeFields{
			Id:   primitive.NewObjectID(),
			Name: name,
		},
		Docs:     make([]*Doc, 0),
		FileRefs: make([]*FileRef, 0),
		Folders:  make([]*Folder, 0),
	}
}

var AbortWalk = errors.New("abort walk")

type MongoPath string

type DirWalker func(folder *Folder, path sharedTypes.DirName) error
type DirWalkerMongo func(parent, folder *Folder, path sharedTypes.DirName, mongoPath MongoPath) error
type TreeWalker func(element TreeElement, path sharedTypes.PathName) error
type TreeWalkerMongo func(parent *Folder, element TreeElement, path sharedTypes.PathName, mongoPath MongoPath) error

var (
	ErrDuplicateNameInFolder = &errors.InvalidStateError{
		Msg: "folder already has entry with given name",
	}
)

const (
	baseMongoPath MongoPath = "rootFolder.0"
)

func (t *Folder) CheckIsUniqueName(needle sharedTypes.Filename) error {
	if t.HasEntry(needle) {
		return ErrDuplicateNameInFolder
	}
	return nil
}

func (t *Folder) HasEntry(needle sharedTypes.Filename) bool {
	entry, _ := t.GetEntry(needle)
	return entry != nil
}

func (t *Folder) GetEntry(needle sharedTypes.Filename) (TreeElement, MongoPath) {
	for i, doc := range t.Docs {
		if doc.Name == needle {
			p := MongoPath(".docs." + strconv.FormatInt(int64(i), 10))
			return doc, p
		}
	}
	for i, file := range t.FileRefs {
		if file.Name == needle {
			p := MongoPath(".fileRefs." + strconv.FormatInt(int64(i), 10))
			return file, p
		}
	}
	for i, folder := range t.Folders {
		if folder.Name == needle {
			p := MongoPath(".folders." + strconv.FormatInt(int64(i), 10))
			return folder, p
		}
	}
	return nil, ""
}

func (t *Folder) Walk(fn TreeWalker) error {
	return ignoreAbort(t.walk(fn, "", walkModeAny))
}

func (t *Folder) WalkDocs(fn TreeWalker) error {
	return ignoreAbort(t.walk(fn, "", walkModeDoc))
}

func (t *Folder) WalkDocsMongo(fn TreeWalkerMongo) error {
	return ignoreAbort(t.walkMongo(fn, "", baseMongoPath, walkModeDoc))
}

func (t *Folder) WalkFiles(fn TreeWalker) error {
	return ignoreAbort(t.walk(fn, "", walkModeFiles))
}

func (t *Folder) WalkFilesMongo(fn TreeWalkerMongo) error {
	return ignoreAbort(t.walkMongo(fn, "", baseMongoPath, walkModeFiles))
}

func (t *Folder) WalkFolders(fn DirWalker) error {
	return ignoreAbort(t.walkDirs(fn, ""))
}

func (t *Folder) WalkFoldersMongo(fn DirWalkerMongo) error {
	return ignoreAbort(t.walkDirsMongo(nil, fn, "", baseMongoPath))
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
		branch := parent.JoinDir(folder.Name)
		if err := folder.walkDirs(fn, branch); err != nil {
			return err
		}
	}
	return nil
}

func (t *Folder) walkDirsMongo(parent *Folder, fn DirWalkerMongo, parentPath sharedTypes.DirName, mongoParent MongoPath) error {
	if err := fn(parent, t, parentPath, mongoParent); err != nil {
		return err
	}
	mongoParent += ".folders."
	for i, folder := range t.Folders {
		branch := parentPath.JoinDir(folder.Name)
		s := MongoPath(strconv.FormatInt(int64(i), 10))
		if err := folder.walkDirsMongo(t, fn, branch, mongoParent+s); err != nil {
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
		branch := parent.JoinDir(folder.Name)
		if err := folder.walk(fn, branch, m); err != nil {
			return err
		}
	}
	return nil
}

func (t *Folder) walkMongo(fn TreeWalkerMongo, parentPath sharedTypes.DirName, mongoParent MongoPath, m walkMode) error {
	if m == walkModeDoc || m == walkModeAny {
		mp := mongoParent + ".docs."
		for i, doc := range t.Docs {
			s := MongoPath(strconv.FormatInt(int64(i), 10))
			if err := fn(t, doc, parentPath.Join(doc.Name), mp+s); err != nil {
				return err
			}
		}
	}
	if m == walkModeFiles || m == walkModeAny {
		mp := mongoParent + ".fileRefs."
		for i, fileRef := range t.FileRefs {
			s := MongoPath(strconv.FormatInt(int64(i), 10))
			if err := fn(t, fileRef, parentPath.Join(fileRef.Name), mp+s); err != nil {
				return err
			}
		}
	}
	mongoParent += ".folders."
	for i, folder := range t.Folders {
		branch := parentPath.JoinDir(folder.Name)
		s := MongoPath(strconv.FormatInt(int64(i), 10))
		if err := folder.walkMongo(fn, branch, mongoParent+s, m); err != nil {
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
	return p.RootFolder[0], nil
}
