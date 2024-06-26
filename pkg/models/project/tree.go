// Golang port of Overleaf
// Copyright (C) 2021-2023 Jakob Ackermann <das7pad@outlook.com>
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
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type TreeNodeKind string

const (
	TreeNodeKindDoc    TreeNodeKind = "doc"
	TreeNodeKindFile   TreeNodeKind = "file"
	TreeNodeKindFolder TreeNodeKind = "folder"
)

type TreeElement interface {
	GetId() sharedTypes.UUID
}

type CommonTreeFields struct {
	Id        sharedTypes.UUID     `json:"_id"`
	Name      sharedTypes.Filename `json:"name"`
	CreatedAt time.Time            `json:"created"`
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

type FileWithParent struct {
	FileRef
	ParentId sharedTypes.UUID
}

type FileRef struct {
	LeafFields

	LinkedFileData *LinkedFileData  `json:"linkedFileData,omitempty"`
	Hash           sharedTypes.Hash `json:"hash,omitempty"`
	Size           int64            `json:"size"`
}

func NewFileRef(name sharedTypes.Filename, hash sharedTypes.Hash, size int64) FileRef {
	f := FileRef{}
	f.Name = name
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

var ErrDuplicateNameInFolder = &errors.InvalidStateError{
	Msg: "folder already has entry with given name",
}

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

func (t *Folder) CountNodes() int {
	n := 1 + len(t.Docs) + len(t.FileRefs)
	for _, f := range t.Folders {
		n += f.CountNodes()
	}
	return n
}

func (t *Folder) WalkFolders(fn func(*Folder) error) error {
	if err := fn(t); err != nil {
		return err
	}
	for i := range t.Folders {
		if err := t.Folders[i].WalkFolders(fn); err != nil {
			return err
		}
	}
	return nil
}

func (t *Folder) PopulateIds(b *sharedTypes.UUIDBatch) {
	t.Id = b.Next()
	for i := range t.Docs {
		t.Docs[i].Id = b.Next()
	}
	for i := range t.FileRefs {
		t.FileRefs[i].Id = b.Next()
	}
	for i := range t.Folders {
		t.Folders[i].PopulateIds(b)
	}
}

func (p *ForTree) GetRootFolder() *Folder {
	t := p.RootFolder
	for i, kind := range p.treeKinds {
		path := sharedTypes.PathName(p.treePaths[i])
		// NOTE: The db has a unique constraint on paths.
		//       We can safely ignore the conflict error here.
		f, _ := t.CreateParents(path.Dir())
		switch kind {
		case TreeNodeKindDoc:
			e := NewDoc(path.Filename())
			e.Id = p.treeIds[i]
			if p.docSnapshots != nil {
				e.Snapshot = p.docSnapshots[i]
			}
			f.Docs = append(f.Docs, e)
		case TreeNodeKindFile:
			e := NewFileRef(path.Filename(), "", 0)
			e.Id = p.treeIds[i]
			if p.createdAts != nil {
				e.CreatedAt = p.createdAts[i].Time
			}
			if p.hashes != nil {
				e.Hash = sharedTypes.Hash(p.hashes[i])
			}
			if p.linkedFileData != nil && p.linkedFileData[i] != nil {
				e.LinkedFileData = p.linkedFileData[i]
			}
			if p.sizes != nil {
				e.Size = p.sizes[i]
			}
			f.FileRefs = append(f.FileRefs, e)
		case TreeNodeKindFolder:
			// NOTE: The paths of folders have a trailing slash in the DB.
			//       When getting f, that slash is removed by the path.Dir()
			//        call and f will have the correct path/name. :)
			f.Id = p.treeIds[i]
		}
	}
	return &t
}

func (p *ForClone) BuildTreeElements() (sharedTypes.PathName, []TreeElement, []sharedTypes.DirName) {
	rootDocId := p.RootDoc.Id
	var rootDocPath sharedTypes.PathName
	elements := make([]TreeElement, 0, len(p.treeKinds))
	folders := make([]sharedTypes.DirName, 0, len(p.treePaths)/3)
	for i, kind := range p.treeKinds {
		path := sharedTypes.PathName(p.treePaths[i])
		switch kind {
		case TreeNodeKindDoc:
			e := NewDoc(path.Filename())
			e.Id = p.treeIds[i]
			e.Path = path
			if p.docSnapshots != nil {
				e.Snapshot = p.docSnapshots[i]
			}
			if e.Id == rootDocId {
				rootDocPath = e.Path
			}
			elements = append(elements, e)
		case TreeNodeKindFile:
			e := NewFileRef(path.Filename(), "", 0)
			e.Id = p.treeIds[i]
			e.Path = path
			if p.createdAts != nil {
				e.CreatedAt = p.createdAts[i].Time
			}
			if p.hashes != nil {
				e.Hash = sharedTypes.Hash(p.hashes[i])
			}
			if p.linkedFileData != nil && p.linkedFileData[i] != nil {
				e.LinkedFileData = p.linkedFileData[i]
			}
			if p.sizes != nil {
				e.Size = p.sizes[i]
			}
			elements = append(elements, e)
		case TreeNodeKindFolder:
			folders = append(folders, sharedTypes.DirName(path))
		}
	}
	return rootDocPath, elements, folders
}
