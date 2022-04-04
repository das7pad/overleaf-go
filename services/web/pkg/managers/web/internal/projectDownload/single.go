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

	"github.com/edgedb/edgedb-go"

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

func (m *manager) getProjectForZip(ctx context.Context, projectId, userId edgedb.UUID, token project.AccessToken) (*project.ForZip, error) {
	for i := 0; i < 10; i++ {
		d, err := m.pm.GetAuthorizationDetails(ctx, projectId, userId, token)
		if err != nil {
			return nil, errors.Tag(err, "cannot check auth")
		}

		if err = m.dum.FlushProject(ctx, projectId); err != nil {
			return nil, errors.Tag(err, "cannot flush project")
		}

		p, err := m.pm.GetForZip(ctx, projectId, d.Epoch)
		if err != nil {
			if err == project.ErrEpochIsNotStable {
				continue
			}
			return nil, errors.Tag(err, "cannot get project")
		}
		return p, nil
	}
	return nil, project.ErrEpochIsNotStable
}

func (m *manager) createProjectZIP(ctx context.Context, request *types.CreateProjectZIPRequest, getBuffer bufferGetter) error {
	userId := request.Session.User.Id
	projectId := request.ProjectId
	token := request.Session.GetAnonTokenAccess(projectId)
	p, errGetProject := m.getProjectForZip(ctx, projectId, userId, token)
	if errGetProject != nil {
		return errGetProject
	}

	buffer, errBuff := getBuffer(sharedTypes.Filename(string(p.Name) + ".zip"))
	if errBuff != nil {
		return errors.Tag(errBuff, "cannot get buffer")
	}
	f := zip.NewWriter(buffer)

	t := p.GetRootFolder()
	{
		err := t.WalkFolders(func(_ *project.Folder, path sharedTypes.DirName) error {
			if _, err := f.Create(path.String() + "/"); err != nil {
				return errors.Tag(err, "create folder: "+path.String())
			}
			return nil
		})
		if err != nil {
			_ = f.Close()
			return err
		}
	}
	err := t.Walk(func(e project.TreeElement, path sharedTypes.PathName) error {
		w, err := f.Create(path.String())
		if err != nil {
			return errors.Tag(err, "create element: "+path.String())
		}
		switch el := e.(type) {
		case *project.Doc:
			_, err = w.Write([]byte(el.Snapshot))
		case *project.FileRef:
			var reader io.ReadCloser
			_, reader, err = m.fm.GetReadStreamForProjectFile(
				ctx, projectId, el.Id, objectStorage.GetOptions{},
			)
			if err != nil {
				return errors.Tag(err, "get file: "+el.Id.String())
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
	errClose := f.Close()
	if err != nil {
		return err
	}
	if errClose != nil {
		return errors.Tag(errClose, "cannot close zip")
	}
	return nil
}
