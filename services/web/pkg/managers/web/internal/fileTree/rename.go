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

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type renameResult struct {
	oldFsPath      sharedTypes.DirEntry
	newFsPath      sharedTypes.DirEntry
	projectVersion sharedTypes.Version
	element        project.TreeElement
}

var (
	errAlreadyRenamed         = &errors.InvalidStateError{Msg: "already renamed"}
	errCannotRenameRootFolder = &errors.ValidationError{Msg: "cannot rename rootFolder"}
)

func ignoreAlreadyRenamedErr(err error) error {
	if err == errAlreadyRenamed {
		return nil
	}
	return err
}

func (m *manager) rename(ctx context.Context, projectId primitive.ObjectID, target project.TreeElement) (*renameResult, error) {
	name := target.GetName()
	targetId := target.GetId()
	r := &renameResult{}

	for i := 0; i < retriesFileTreeOperation; i++ {
		t, v, err := m.pm.GetProjectRootFolder(ctx, projectId)
		if err != nil {
			return nil, errors.Tag(err, "cannot get project")
		}

		var parent *project.Folder
		var mongoPath project.MongoPath
		switch target.(type) {
		case *project.Doc, *project.FileRef:
			var walkFn func(mongo project.TreeWalkerMongo) error
			if _, ok := target.(*project.Doc); ok {
				walkFn = t.WalkDocsMongo
			} else {
				walkFn = t.WalkFilesMongo
			}
			err = walkFn(func(f *project.Folder, element project.TreeElement, path sharedTypes.PathName, mPath project.MongoPath) error {
				if element.GetId() == targetId {
					parent = f
					r.element = element
					r.oldFsPath = path
					mongoPath = mPath
					return project.AbortWalk
				}
				return nil
			})
		case *project.Folder:
			if targetId == t.Id {
				return nil, errCannotRenameRootFolder
			}
			err = t.WalkFoldersMongo(func(p, f *project.Folder, path sharedTypes.DirName, mPath project.MongoPath) error {
				if f.GetId() == targetId {
					parent = p
					r.element = f
					r.oldFsPath = path
					mongoPath = mPath
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
		if r.element.GetName() == name {
			return nil, errAlreadyRenamed
		}

		if err = parent.CheckIsUniqueName(name); err != nil {
			return nil, err
		}

		r.element.SetName(name)
		switch target.(type) {
		case *project.Doc, *project.FileRef:
			r.newFsPath = r.oldFsPath.Dir().Join(name)
		case *project.Folder:
			r.newFsPath = r.oldFsPath.Dir().JoinDir(name)
		}
		err = m.pm.RenameTreeElement(ctx, projectId, v, mongoPath, name)
		if err != nil {
			if err == project.ErrVersionChanged {
				continue
			}
			return nil, errors.Tag(err, "cannot rename element in tree")
		}
		r.projectVersion = v + 1
		return r, nil
	}
	return nil, project.ErrVersionChanged
}
