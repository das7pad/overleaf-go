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

package fileTree

import (
	"context"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type moveResult struct {
	oldFsPath      sharedTypes.DirEntry
	newFsPath      sharedTypes.DirEntry
	projectVersion sharedTypes.Version
	element        project.TreeElement
}

var (
	errAlreadyMoved         = &errors.InvalidStateError{Msg: "already moved"}
	errCannotMoveIntoItself = &errors.ValidationError{Msg: "cannot move into itself"}
	errCannotMoveRootFolder = &errors.ValidationError{Msg: "cannot move rootFolder"}
)

func ignoreAlreadyMovedErr(err error) error {
	if err == errAlreadyMoved {
		return nil
	}
	return err
}

func (m *manager) move(ctx context.Context, projectId, targetId edgedb.UUID, source project.TreeElement) (*moveResult, error) {
	sourceId := source.GetId()
	if sourceId == targetId {
		return nil, errCannotMoveIntoItself
	}
	r := &moveResult{}

	for i := 0; i < retriesFileTreeOperation; i++ {
		t, v, err := m.pm.GetProjectRootFolder(ctx, projectId)
		if err != nil {
			return nil, errors.Tag(err, "cannot get project")
		}

		var parent *project.Folder
		var fromMongoPath, toMongoPath project.MongoPath
		switch source.(type) {
		case *project.Doc, *project.FileRef:
			var walkFn func(mongo project.TreeWalkerMongo) error
			if _, ok := source.(*project.Doc); ok {
				walkFn = t.WalkDocsMongo
			} else {
				walkFn = t.WalkFilesMongo
			}
			err = walkFn(func(f *project.Folder, element project.TreeElement, path sharedTypes.PathName, mPath project.MongoPath) error {
				if element.GetId() == sourceId {
					parent = f
					r.element = element
					source = element
					r.oldFsPath = path
					fromMongoPath = mPath
					return project.AbortWalk
				}
				return nil
			})
		case *project.Folder:
			if sourceId == t.Id {
				return nil, errCannotMoveRootFolder
			}
			err = t.WalkFoldersMongo(func(p, f *project.Folder, path sharedTypes.DirName, mPath project.MongoPath) error {
				if f.Id == sourceId {
					parent = p
					r.element = f
					source = f
					r.oldFsPath = path
					fromMongoPath = mPath

					targetIsChild := false
					err = f.WalkFolders(func(child *project.Folder, _ sharedTypes.DirName) error {
						if child.Id == targetId {
							targetIsChild = true
							return project.AbortWalk
						}
						return nil
					})
					if err != nil {
						return err
					}
					if targetIsChild {
						return errCannotMoveIntoItself
					}

					return project.AbortWalk
				}
				return nil
			})

		}
		if err != nil {
			return nil, err
		}
		if r.element == nil {
			return nil, errors.Tag(&errors.NotFoundError{}, "element not found")
		}
		if parent.Id == targetId {
			return nil, errAlreadyMoved
		}

		var target *project.Folder
		err = t.WalkFoldersMongo(func(p, f *project.Folder, path sharedTypes.DirName, mPath project.MongoPath) error {
			if f.Id == targetId {
				target = f
				if _, isFolder := source.(*project.Folder); isFolder {
					r.newFsPath = path.JoinDir(source.GetName())
				} else {
					r.newFsPath = path.Join(source.GetName())
				}
				toMongoPath = mPath
				return project.AbortWalk
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		if target == nil {
			return nil, errors.Tag(&errors.NotFoundError{}, "target not found")
		}

		if err = target.CheckIsUniqueName(source.GetName()); err != nil {
			return nil, err
		}

		err = m.pm.MoveTreeElement(ctx, projectId, v, fromMongoPath, toMongoPath, r.element)
		if err != nil {
			if err == project.ErrVersionChanged {
				continue
			}
			return nil, errors.Tag(err, "cannot move element in tree")
		}
		r.projectVersion = v + 1
		return r, nil
	}
	return nil, project.ErrVersionChanged
}
