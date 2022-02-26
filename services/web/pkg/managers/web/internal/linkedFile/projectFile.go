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

package linkedFile

import (
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

func (m *manager) fromProjectFile(ctx context.Context, request *types.CreateLinkedFileRequest) error {
	sourceProjectId := request.Parameter.SourceProjectId
	userId := request.UserId
	p, err := m.pm.GetTreeAndAuth(ctx, sourceProjectId, userId)
	if err != nil {
		return err
	}
	if _, err = p.GetPrivilegeLevelAuthenticated(userId); err != nil {
		return err
	}
	t, err := p.GetRootFolder()
	if err != nil {
		return err
	}

	var e project.TreeElement
	err = t.Walk(func(element project.TreeElement, path sharedTypes.PathName) error {
		if path == request.Parameter.SourceEntityPath {
			e = element
			return project.AbortWalk
		}
		return nil
	})
	if err != nil {
		return err
	}
	if e == nil {
		return &errors.NotFoundError{}
	}

	f, err := os.CreateTemp("", "linked-file")
	if err != nil {
		return errors.Tag(err, "cannot create temp file")
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()

	var size int64
	switch e.(type) {
	case *project.Doc:
		d, err2 := m.dum.GetDoc(ctx, sourceProjectId, e.GetId(), -1)
		if err2 != nil {
			return errors.Tag(err2, "cannot get doc")
		}
		n, err2 := f.WriteString(string(d.Snapshot))
		if err2 != nil {
			return errors.Tag(err2, "cannot buffer doc")
		}
		size = int64(n)
	case *project.FileRef:
		_, reader, err2 := m.fm.GetReadStreamForProjectFile(
			ctx, sourceProjectId, e.GetId(), objectStorage.GetOptions{},
		)
		if err2 != nil {
			return errors.Tag(err2, "cannot get file")
		}
		size, err = io.Copy(f, reader)
		_ = reader.Close()
		if err != nil {
			return errors.Tag(err, "cannot buffer file")
		}
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return errors.Tag(err, "cannot reset buffer to start")
	}

	return m.ftm.UploadFile(ctx, &types.UploadFileRequest{
		ProjectId:      request.ProjectId,
		UserId:         request.UserId,
		ParentFolderId: request.ParentFolderId,
		LinkedFileData: request.LinkedFileData(),
		UploadDetails: types.UploadDetails{
			File:     f,
			FileName: request.Name,
			Size:     size,
		},
	})
}

func (m *manager) refreshProjectFile(ctx context.Context, r *types.RefreshLinkedFileRequest) error {
	id, err := edgedb.ParseUUID(r.File.LinkedFileData.SourceProjectId)
	if err != nil {
		return &errors.InvalidStateError{Msg: "corrupt source project id"}
	}
	return m.fromProjectFile(ctx, &types.CreateLinkedFileRequest{
		UserId:         r.UserId,
		ProjectId:      r.ProjectId,
		ParentFolderId: r.ParentFolderId,
		Name:           r.File.Name,
		Provider:       r.File.LinkedFileData.Provider,
		Parameter: types.CreateLinkedFileProviderParameter{
			SourceProjectId: id,
			SourceEntityPath: sharedTypes.PathName(
				r.File.LinkedFileData.SourceEntityPath,
			),
		},
	})
}
