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

package projectDownload

import (
	"archive/zip"
	"context"
	"io"
	"os"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
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

func (m *manager) getProjectForZip(ctx context.Context, projectId, userId sharedTypes.UUID, token project.AccessToken) (*project.ForZip, error) {
	_, err := m.pm.GetAuthorizationDetails(ctx, projectId, userId, token)
	if err != nil {
		return nil, errors.Tag(err, "cannot check auth")
	}

	if err = m.dum.FlushProject(ctx, projectId); err != nil {
		return nil, errors.Tag(err, "cannot flush project")
	}

	p, err := m.pm.GetForZip(ctx, projectId, userId, token)
	if err != nil {
		return nil, errors.Tag(err, "cannot get project")
	}
	return p, nil
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
	z := zip.NewWriter(buffer)

	t := p.GetRootFolder()
	err := t.WalkFolders(func(f *project.Folder) error {
		if _, err := z.Create(f.Path.String() + "/"); err != nil {
			return errors.Tag(err, "create folder: "+f.Path.String())
		}

		for _, d := range f.Docs {
			path := f.Path.Join(d.Name).String()
			w, err := z.Create(path)
			if err != nil {
				return errors.Tag(err, "create doc: "+path)
			}
			if _, err = w.Write([]byte(d.Snapshot)); err != nil {
				return errors.Tag(err, "write doc: "+path)
			}
		}

		for _, fileRef := range f.FileRefs {
			path := f.Path.Join(fileRef.Name).String()
			w, err := z.Create(path)
			if err != nil {
				return errors.Tag(err, "create file: "+path)
			}
			_, reader, err := m.fm.GetReadStreamForProjectFile(
				ctx, projectId, fileRef.Id,
			)
			if err != nil {
				return errors.Tag(err, "get file: "+fileRef.Id.String())
			}
			_, errCopy := io.Copy(w, reader)
			errClose := reader.Close()
			if errCopy != nil {
				return errors.Tag(errCopy, "write file: "+path)
			}
			if errClose != nil {
				return errors.Tag(errClose, "close file: "+path)
			}
		}
		return nil
	})
	errClose := z.Close()
	if err != nil {
		return err
	}
	if errClose != nil {
		return errors.Tag(errClose, "cannot close zip")
	}
	return nil
}
