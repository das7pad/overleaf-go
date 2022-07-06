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

package project

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/lib/pq"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type TreeElement interface {
	GetId() sharedTypes.UUID
}

type CommonTreeFields struct {
	Id   sharedTypes.UUID     `json:"_id"`
	Name sharedTypes.Filename `json:"name"`
}

type LeafFields struct {
	CommonTreeFields
	Path sharedTypes.PathName `json:"-"`
}

func (c CommonTreeFields) GetId() sharedTypes.UUID {
	return c.Id
}

type RootDoc struct {
	Doc
}

type DocWithParent struct {
	Doc
	Parent IdField
}

type Doc struct {
	LeafFields
	Snapshot string              `json:"snapshot"`
	Version  sharedTypes.Version `json:"version"`
}

func NewDoc(name sharedTypes.Filename) Doc {
	d := Doc{}
	d.Name = name
	return d
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
	Provider             LinkedFileProvider   `json:"provider"`
	SourceProjectId      sharedTypes.UUID     `json:"source_project_id,omitempty"`
	SourceEntityPath     sharedTypes.PathName `json:"source_entity_path,omitempty"`
	SourceOutputFilePath sharedTypes.PathName `json:"source_output_file_path,omitempty"`
	URL                  string               `json:"url,omitempty"`
}

func (d *LinkedFileData) Scan(x interface{}) error {
	if x == nil {
		return nil
	}
	return json.Unmarshal(x.([]byte), d)
}

func (d *LinkedFileData) Value() (driver.Value, error) {
	if d == nil || d.Provider == "" {
		return nil, nil
	}
	blob, err := json.Marshal(d)
	return string(blob), err
}

type FileWithParent struct {
	FileRef
	ParentId sharedTypes.UUID
}

type FileRef struct {
	LeafFields

	LinkedFileData *LinkedFileData  `json:"linkedFileData,omitempty"`
	Hash           sharedTypes.Hash `json:"hash,omitempty"`
	Created        time.Time        `json:"created"`
	Size           int64            `json:"size"`
}

func NewFileRef(name sharedTypes.Filename, hash sharedTypes.Hash, size int64) FileRef {
	f := FileRef{}
	f.Name = name
	f.Created = time.Now().Truncate(time.Microsecond)
	f.Hash = hash
	f.Size = size
	return f
}

type Folder struct {
	CommonTreeFields
	Path sharedTypes.DirName `json:"-"`

	Docs     []Doc     `json:"docs"`
	FileRefs []FileRef `json:"fileRefs"`
	Folders  []Folder  `json:"folders"`
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
		Docs:     make([]Doc, 0, 10),
		FileRefs: make([]FileRef, 0, 10),
		Folders:  make([]Folder, 0, 10),
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

func (t *Folder) WalkFull(fn func(e TreeElement)) {
	fnDir := func(f *Folder, path sharedTypes.DirName) error {
		fn(f)
		return nil
	}
	fnTree := func(e TreeElement, path sharedTypes.PathName) error {
		fn(e)
		return nil
	}
	_ = t.walkDirs(fnDir, t.Path)
	_ = t.walk(fnTree, t.Path, walkModeAny)
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

func (p *ForTree) GetRootFolder() *Folder {
	t := p.RootFolder
	for i, kind := range p.treeKinds {
		path := sharedTypes.PathName(p.treePaths[i])
		// NOTE: The db has a unique constraint on paths.
		//       We can safely ignore the conflict error here.
		f, _ := t.CreateParents(path.Dir())
		switch kind {
		case "doc":
			e := NewDoc(path.Filename())
			e.Id = p.treeIds[i]
			if p.docSnapshots != nil {
				e.Snapshot = p.docSnapshots[i]
			}
			f.Docs = append(f.Docs, e)
		case "file":
			e := NewFileRef(path.Filename(), "", 0)
			e.Id = p.treeIds[i]
			if p.createdAts != nil {
				s := string(p.createdAts[i])
				e.Created, _ = pq.ParseTimestamp(nil, s)
			}
			if p.hashes != nil {
				e.Hash = sharedTypes.Hash(p.hashes[i])
			}
			if p.linkedFileData != nil && p.linkedFileData[i].Provider != "" {
				e.LinkedFileData = &p.linkedFileData[i]
			}
			if p.sizes != nil {
				e.Size = p.sizes[i]
			}
			f.FileRefs = append(f.FileRefs, e)
		case "folder":
			// NOTE: The paths of folders have a trailing slash in the DB.
			//       When getting f, that slash is removed by the path.Dir()
			//        call and f will have the correct path/name. :)
			f.Id = p.treeIds[i]
		}
	}
	return &t
}

func (p *ForTree) GetDocsAndFiles() ([]Doc, []FileRef) {
	var docs []Doc
	var files []FileRef
	for i, kind := range p.treeKinds {
		path := sharedTypes.PathName(p.treePaths[i])
		switch kind {
		case "doc":
			e := NewDoc(path.Filename())
			e.Id = p.treeIds[i]
			e.Path = path
			if p.docSnapshots != nil {
				e.Snapshot = p.docSnapshots[i]
			}
			docs = append(docs, e)
		case "file":
			e := NewFileRef(path.Filename(), "", 0)
			e.Id = p.treeIds[i]
			e.Path = path
			if p.createdAts != nil {
				s := string(p.createdAts[i])
				e.Created, _ = pq.ParseTimestamp(nil, s)
			}
			if p.hashes != nil {
				e.Hash = sharedTypes.Hash(p.hashes[i])
			}
			if p.linkedFileData != nil && p.linkedFileData[i].Provider != "" {
				e.LinkedFileData = &p.linkedFileData[i]
			}
			if p.sizes != nil {
				e.Size = p.sizes[i]
			}
			files = append(files, e)
		}
	}
	return docs, files
}
