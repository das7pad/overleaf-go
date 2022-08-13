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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgtype"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/m2pq"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type TreeElement interface {
	GetId() primitive.ObjectID
}

type CommonTreeFields struct {
	Id   primitive.ObjectID   `json:"_id" bson:"_id"`
	Name sharedTypes.Filename `json:"name" bson:"name"`
}

func (c CommonTreeFields) GetId() primitive.ObjectID {
	return c.Id
}

type Doc struct {
	CommonTreeFields `bson:"inline"`

	Snapshot string
	Version  sharedTypes.Version
}

type LinkedFileProvider string

type LinkedFileData struct {
	Provider             LinkedFileProvider `json:"provider" bson:"provider"`
	SourceProjectId      string             `json:"source_project_id,omitempty" bson:"source_project_id,omitempty"`
	SourceEntityPath     string             `json:"source_entity_path,omitempty" bson:"source_entity_path,omitempty"`
	SourceOutputFilePath string             `json:"source_output_file_path,omitempty" bson:"source_output_file_path,omitempty"`
	URL                  string             `json:"url,omitempty" bson:"url,omitempty"`
}

func (d *LinkedFileData) Migrate() error {
	if d == nil || d.Provider == "" {
		return nil
	}

	// The NodeJS implementation stored these as absolute paths.
	if d.SourceEntityPath != "" {
		d.SourceEntityPath = strings.TrimPrefix(
			d.SourceEntityPath, "/",
		)
	}
	if d.SourceOutputFilePath != "" {
		d.SourceOutputFilePath = strings.TrimPrefix(
			d.SourceOutputFilePath, "/",
		)
	}

	if d.SourceProjectId != "" {
		id, err := m2pq.ParseID(d.SourceProjectId)
		if err != nil {
			return errors.Tag(err, "invalid source project id")
		}
		d.SourceProjectId = id.String()
	}
	return nil
}

func (d *LinkedFileData) EncodeBinary(ci *pgtype.ConnInfo, buf []byte) ([]byte, error) {
	if d == nil || d.Provider == "" {
		return nil, nil
	}
	blob, err := json.Marshal(d)
	if err != nil {
		return nil, errors.Tag(err, "serialize LinkedFileData")
	}
	return pgtype.JSON{
		Bytes:  blob,
		Status: pgtype.Present,
	}.EncodeBinary(ci, buf)
}

type FileRef struct {
	CommonTreeFields `bson:"inline"`

	LinkedFileData *LinkedFileData  `json:"linkedFileData,omitempty" bson:"linkedFileData,omitempty"`
	Hash           sharedTypes.Hash `json:"hash" bson:"hash"`
	Created        time.Time        `json:"created" bson:"created"`
	Size           *int64           `json:"size" bson:"size"`
}

type Folder struct {
	CommonTreeFields `bson:"inline"`

	Docs     []*Doc     `json:"docs" bson:"docs"`
	FileRefs []*FileRef `json:"fileRefs" bson:"fileRefs"`
	Folders  []*Folder  `json:"folders" bson:"folders"`
}

type MongoPath string

type DirWalker func(folder *Folder, path sharedTypes.DirName) error

type DirWalkerMongo func(parent, folder *Folder, path sharedTypes.DirName, mongoPath MongoPath) error

type TreeWalker func(element TreeElement, path sharedTypes.PathName) error

type TreeWalkerMongo func(parent *Folder, element TreeElement, path sharedTypes.PathName, mongoPath MongoPath) error

func (t *Folder) CountNodes() int {
	n := 1 + len(t.Docs) + len(t.FileRefs)
	for _, f := range t.Folders {
		n += f.CountNodes()
	}
	return n
}

func (t *Folder) WalkFiles(fn TreeWalker) error {
	return t.walk(fn, "", walkModeFiles)
}

func (t *Folder) WalkFolders(fn DirWalker) error {
	return t.walkDirs(fn, "")
}

type walkMode int

const (
	walkModeDoc walkMode = iota
	walkModeFiles
	walkModeAny
)

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
