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

package projectDownload

import (
	"archive/zip"
	"context"
	"io"
	"os"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/objectStorage"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) CreateProjectZIP(ctx context.Context, request *types.CreateProjectZIPRequest, response *types.CreateProjectZIPResponse) error {
	buffer, errCreateBuffer := os.CreateTemp("", "zip-download")
	if errCreateBuffer != nil {
		return errors.Tag(errCreateBuffer, "cannot create buffer")
	}
	response.FSPath = buffer.Name()

	errCreate := m.createProjectZIP(ctx, request, func(filename sharedTypes.Filename) (io.Writer, error) {
		response.Filename = filename
		return buffer, nil
	})

	errClose := buffer.Close()
	if errCreate != nil {
		return errCreate
	}
	if errClose != nil {
		return errors.Tag(errClose, "cannot close buffer")
	}
	return nil
}

type bufferGetter func(filename sharedTypes.Filename) (io.Writer, error)

func (m *manager) createProjectZIP(ctx context.Context, request *types.CreateProjectZIPRequest, getBuffer bufferGetter) error {
	userId := request.Session.User.Id
	projectId := request.ProjectId
	p, err := m.pm.GetTreeAndAuth(ctx, projectId, userId)
	if err != nil {
		return errors.Tag(err, "cannot get project")
	}
	_, err = p.GetPrivilegeLevel(
		userId, request.Session.GetAnonTokenAccess(projectId),
	)
	if err != nil {
		return err
	}
	t, err := p.GetRootFolder()
	if err != nil {
		return err
	}

	if err = m.dum.FlushProject(ctx, projectId); err != nil {
		return errors.Tag(err, "cannot flush project")
	}
	docs, err := m.dm.GetAllDocContents(ctx, projectId)
	if err != nil {
		return errors.Tag(err, "cannot get docs")
	}

	docsById := make(map[primitive.ObjectID][]byte, len(docs))
	for i, d := range docs {
		docsById[d.Id] = []byte(string(d.Lines.ToSnapshot()))
		docs[i] = nil
	}

	buffer, err := getBuffer(sharedTypes.Filename(string(p.Name) + ".zip"))
	if err != nil {
		return errors.Tag(err, "cannot get buffer")
	}

	f := zip.NewWriter(buffer)
	err = t.WalkFolders(func(_ *project.Folder, path sharedTypes.DirName) error {
		if _, err = f.Create(path.String() + "/"); err != nil {
			return errors.Tag(err, "create folder: "+path.String())
		}
		return nil
	})
	if err != nil {
		return err
	}
	err = t.Walk(func(e project.TreeElement, path sharedTypes.PathName) error {
		w, errCreate := f.Create(path.String())
		if errCreate != nil {
			return errors.Tag(errCreate, "create element: "+path.String())
		}
		switch e.(type) {
		case *project.Doc:
			b, exists := docsById[e.GetId()]
			if !exists {
				return &errors.InvalidStateError{
					Msg: "missing doc: " + e.GetId().Hex(),
				}
			}
			_, err = w.Write(b)
		case *project.FileRef:
			var reader io.ReadCloser
			_, reader, err = m.fm.GetReadStreamForProjectFile(
				ctx, projectId, e.GetId(), objectStorage.GetOptions{},
			)
			if err != nil {
				return errors.Tag(err, "get file: "+e.GetId().Hex())
			}
			_, err = io.Copy(w, reader)
			errClose := reader.Close()
			if err == nil && errClose != nil {
				err = errors.Tag(errClose, "cannot close file")
			}
		}
		if err != nil {
			return errors.Tag(err, "cannot write: "+path.String())
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		err = errors.Tag(err, "cannot close zip")
	}
	return nil
}
