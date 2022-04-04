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
}

type CommonTreeFields struct {
	Id   edgedb.UUID          `json:"_id" edgedb:"id"`
	Name sharedTypes.Filename `json:"name" edgedb:"name"`
}

func (c CommonTreeFields) GetId() edgedb.UUID {
	return c.Id
}

type RootDoc struct {
	edgedb.Optional
	Doc `edgedb:"$inline"`
}

type DocWithParent struct {
	Doc    `edgedb:"$inline"`
	Parent IdField `edgedb:"parent"`
}

type Doc struct {
	CommonTreeFields `edgedb:"$inline"`
	Path             sharedTypes.PathName `edgedb:"resolved_path"`
	Size             int64                `json:"size" edgedb:"size"`
	Snapshot         string               `json:"snapshot" edgedb:"snapshot"`
	Version          sharedTypes.Version  `json:"version" edgedb:"version"`
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
	edgedb.Optional
	Provider             LinkedFileProvider   `json:"provider" edgedb:"provider"`
	SourceProjectId      string               `json:"source_project_id,omitempty" edgedb:"source_project_id"`
	SourceEntityPath     sharedTypes.PathName `json:"source_entity_path,omitempty" edgedb:"source_entity_path"`
	SourceOutputFilePath sharedTypes.PathName `json:"source_output_file_path,omitempty" edgedb:"source_output_file_path"`
	URL                  string               `json:"url,omitempty" edgedb:"url"`
}

type FileWithParent struct {
	FileRef `edgedb:"$inline"`
	Parent  IdField `edgedb:"parent"`
}

type FileRef struct {
	CommonTreeFields `edgedb:"$inline"`
	Path             sharedTypes.PathName `edgedb:"resolved_path"`

	LinkedFileData LinkedFileData   `json:"linkedFileData,omitempty" edgedb:"linked_file_data"`
	Hash           sharedTypes.Hash `json:"hash" edgedb:"hash"`
	Created        time.Time        `json:"created" edgedb:"created_at"`
	Size           int64            `json:"size" edgedb:"size"`
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
	Path             sharedTypes.DirName `json:"-" edgedb:"path"`

	Docs     []Doc     `json:"docs" edgedb:"docs"`
	FileRefs []FileRef `json:"fileRefs" edgedb:"files"`
	Folders  []Folder  `json:"folders" edgedb:"folders"`
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
	entry := parent.GetEntry(name)
	if entry == nil {
		folder := NewFolder(name)
		folder.Path = parent.Path.JoinDir(name)
		parent.Folders = append(parent.Folders, folder)
		return &parent.Folders[len(parent.Folders)-1], nil
	}
	if folder, ok := entry.(*Folder); ok {
		return folder, nil
	}
	return nil, &errors.ValidationError{Msg: "conflicting paths"}
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

type DirWalker func(folder *Folder, path sharedTypes.DirName) error
type TreeWalker func(element TreeElement, path sharedTypes.PathName) error

var (
	ErrDuplicateNameInFolder = &errors.InvalidStateError{
		Msg: "folder already has entry with given name",
	}
)

func (t *Folder) HasEntry(needle sharedTypes.Filename) bool {
	return t.GetEntry(needle) != nil
}

func (t *Folder) GetEntry(needle sharedTypes.Filename) TreeElement {
	for i, doc := range t.Docs {
		if doc.Name == needle {
			return &t.Docs[i]
		}
	}
	for i, file := range t.FileRefs {
		if file.Name == needle {
			return &t.FileRefs[i]
		}
	}
	for i, folder := range t.Folders {
		if folder.Name == needle {
			return &t.Folders[i]
		}
	}
	return nil
}

func (t *Folder) GetDoc(needle sharedTypes.Filename) *Doc {
	for i, doc := range t.Docs {
		if doc.Name == needle {
			return &t.Docs[i]
		}
	}
	return nil
}

func (t *Folder) GetFile(needle sharedTypes.Filename) *FileRef {
	for i, file := range t.FileRefs {
		if file.Name == needle {
			return &t.FileRefs[i]
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

func (t *Folder) WalkFiles(fn TreeWalker) error {
	return ignoreAbort(t.walk(fn, t.Path, walkModeFiles))
}

func (t *Folder) WalkFolders(fn DirWalker) error {
	return ignoreAbort(t.walkDirs(fn, t.Path))
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
	for i, folder := range t.Folders {
		branch := parent.JoinDir(folder.Name)
		if err := t.Folders[i].walkDirs(fn, branch); err != nil {
			return err
		}
	}
	return nil
}

func (t *Folder) walk(fn TreeWalker, parent sharedTypes.DirName, m walkMode) error {
	if m == walkModeDoc || m == walkModeAny {
		for i, doc := range t.Docs {
			if err := fn(&t.Docs[i], parent.Join(doc.Name)); err != nil {
				return err
			}
		}
	}
	if m == walkModeFiles || m == walkModeAny {
		for i, fileRef := range t.FileRefs {
			err := fn(&t.FileRefs[i], parent.Join(fileRef.Name))
			if err != nil {
				return err
			}
		}
	}
	for i, folder := range t.Folders {
		branch := parent.JoinDir(folder.Name)
		if err := t.Folders[i].walk(fn, branch, m); err != nil {
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
