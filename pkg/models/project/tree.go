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

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type RootFolder struct {
	edgedb.Optional
	Folder `edgedb:"$inline"`
}

type TreeElement interface {
	GetId() edgedb.UUID
	GetName() sharedTypes.Filename
	SetName(name sharedTypes.Filename)
	FieldNameInFolder() MongoPath
}

type CommonTreeFields struct {
	Id   edgedb.UUID          `json:"_id" edgedb:"id"`
	Name sharedTypes.Filename `json:"name" edgedb:"name"`
}

func (c CommonTreeFields) GetId() edgedb.UUID {
	return c.Id
}

func (c CommonTreeFields) GetName() sharedTypes.Filename {
	return c.Name
}

func (c *CommonTreeFields) SetName(name sharedTypes.Filename) {
	c.Name = name
}

type RootDoc struct {
	edgedb.Optional
	DocWithParent `edgedb:"$inline"`
}

type DocWithParent struct {
	Doc    `edgedb:"$inline"`
	Parent Folder `edgedb:"parent"`
}

func (d *DocWithParent) GetPath() sharedTypes.PathName {
	return d.Parent.Path.Join(d.Name)
}

type Doc struct {
	CommonTreeFields `edgedb:"$inline"`
	Size             int64               `json:"size" edgedb:"size"`
	Snapshot         string              `json:"snapshot" edgedb:"snapshot"`
	Version          sharedTypes.Version `json:"version" edgedb:"version"`
}

func (d *Doc) FieldNameInFolder() MongoPath {
	return "docs"
}

func NewDoc(name sharedTypes.Filename) Doc {
	return Doc{CommonTreeFields: CommonTreeFields{
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
	Provider             LinkedFileProvider `json:"provider" edgedb:"provider"`
	SourceProjectId      string             `json:"source_project_id,omitempty" edgedb:"source_project_id"`
	SourceEntityPath     string             `json:"source_entity_path,omitempty" edgedb:"source_entity_path"`
	SourceOutputFilePath string             `json:"source_output_file_path,omitempty" edgedb:"source_output_file_path"`
	URL                  string             `json:"url,omitempty" edgedb:"url"`
}

type TreeElementInProject struct {
	edgedb.Optional
	CommonTreeFields `edgedb:"$inline"`
	Project          IdField `edgedb:"project"`
}

type OptionalIdField struct {
	edgedb.Optional
	IdField `edgedb:"$inline"`
}

type FileRef struct {
	CommonTreeFields `edgedb:"$inline"`

	LinkedFileData *LinkedFileData  `json:"linkedFileData,omitempty" edgedb:"linkedFileData"`
	Hash           sharedTypes.Hash `json:"hash" edgedb:"hash"`
	Created        time.Time        `json:"created" edgedb:"created_at"`
	Size           int64            `json:"size" edgedb:"size"`

	// TODO: pointer
	SourceElement        TreeElementInProject `json:"source_element,omitempty" edgedb:"source_element"`
	SourceOutputFilePath edgedb.OptionalStr   `json:"source_path,omitempty" edgedb:"source_path"`
	SourceProject        OptionalIdField      `json:"source_project,omitempty" edgedb:"source_project"`
	URL                  edgedb.OptionalStr   `json:"url,omitempty" edgedb:"url"`
}

func (f *FileRef) FieldNameInFolder() MongoPath {
	return "fileRefs"
}

func NewFileRef(name sharedTypes.Filename, hash sharedTypes.Hash, size int64) FileRef {
	return FileRef{
		CommonTreeFields: CommonTreeFields{
			Name: name,
		},
		Created: time.Now().UTC(),
		Hash:    hash,
		Size:    size,
	}
}

type Folder struct {
	CommonTreeFields `edgedb:"$inline"`
	Path             sharedTypes.DirName `edgedb:"path"`

	Docs     []Doc     `json:"docs" edgedb:"docs"`
	FileRefs []FileRef `json:"fileRefs" edgedb:"files"`
	Folders  []Folder  `json:"folders" edgedb:"folders"`
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
		folder.Path = parent.Path.JoinDir(name)
		parent.Folders = append(parent.Folders, folder)
		return &folder, nil
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

func NewFolder(name sharedTypes.Filename) Folder {
	return Folder{
		CommonTreeFields: CommonTreeFields{
			Name: name,
		},
		Docs:     make([]Doc, 0),
		FileRefs: make([]FileRef, 0),
		Folders:  make([]Folder, 0),
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
			return &doc, p
		}
	}
	for i, file := range t.FileRefs {
		if file.Name == needle {
			p := MongoPath(".fileRefs." + strconv.FormatInt(int64(i), 10))
			return &file, p
		}
	}
	for i, folder := range t.Folders {
		if folder.Name == needle {
			p := MongoPath(".folders." + strconv.FormatInt(int64(i), 10))
			return &folder, p
		}
	}
	return nil, ""
}

func (t *Folder) GetDoc(needle sharedTypes.Filename) *Doc {
	for _, doc := range t.Docs {
		if doc.Name == needle {
			return &doc
		}
	}
	return nil
}

func (t *Folder) GetFile(needle sharedTypes.Filename) *FileRef {
	for _, file := range t.FileRefs {
		if file.Name == needle {
			return &file
		}
	}
	return nil
}

func (t *Folder) Walk(fn TreeWalker) error {
	return ignoreAbort(t.walk(fn, t.Path, walkModeAny))
}

func (t *Folder) WalkDocs(fn TreeWalker) error {
	return ignoreAbort(t.walk(fn, t.Path, walkModeDoc))
}

func (t *Folder) WalkDocsMongo(fn TreeWalkerMongo) error {
	return ignoreAbort(t.walkMongo(fn, t.Path, baseMongoPath, walkModeDoc))
}

func (t *Folder) WalkFiles(fn TreeWalker) error {
	return ignoreAbort(t.walk(fn, t.Path, walkModeFiles))
}

func (t *Folder) WalkFilesMongo(fn TreeWalkerMongo) error {
	return ignoreAbort(t.walkMongo(fn, t.Path, baseMongoPath, walkModeFiles))
}

func (t *Folder) WalkFolders(fn DirWalker) error {
	return ignoreAbort(t.walkDirs(fn, t.Path))
}

func (t *Folder) WalkFoldersMongo(fn DirWalkerMongo) error {
	return ignoreAbort(t.walkDirsMongo(nil, fn, t.Path, baseMongoPath))
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
			if err := fn(&doc, parent.Join(doc.Name)); err != nil {
				return err
			}
		}
	}
	if m == walkModeFiles || m == walkModeAny {
		for _, fileRef := range t.FileRefs {
			if err := fn(&fileRef, parent.Join(fileRef.Name)); err != nil {
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
			if err := fn(t, &doc, parentPath.Join(doc.Name), mp+s); err != nil {
				return err
			}
		}
	}
	if m == walkModeFiles || m == walkModeAny {
		mp := mongoParent + ".fileRefs."
		for i, fileRef := range t.FileRefs {
			s := MongoPath(strconv.FormatInt(int64(i), 10))
			if err := fn(t, &fileRef, parentPath.Join(fileRef.Name), mp+s); err != nil {
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

func (p *ForTree) GetRootFolder() *Folder {
	if p.rootFolderResolved {
		return &p.RootFolder.Folder
	}

	lookup := make(map[edgedb.UUID]Folder, len(p.Folders))
	for _, folder := range p.Folders {
		lookup[folder.Id] = folder
	}

	for _, folder := range p.Folders {
		for i, f := range folder.Folders {
			folder.Folders[i] = lookup[f.Id]
		}
	}

	for i, f := range p.RootFolder.Folders {
		p.RootFolder.Folders[i] = lookup[f.Id]
	}

	p.rootFolderResolved = true
	p.Folders = nil
	return &p.RootFolder.Folder
}
