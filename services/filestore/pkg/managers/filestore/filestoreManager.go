// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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

package filestore

import (
	"context"
	"io"
	"net/url"

	"github.com/das7pad/overleaf-go/pkg/objectStorage"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	CopyProjectFile(ctx context.Context, srcProjectId sharedTypes.UUID, srcFileId sharedTypes.UUID, destProjectId sharedTypes.UUID, destFileId sharedTypes.UUID) error
	DeleteProjectFile(ctx context.Context, projectId sharedTypes.UUID, fileId sharedTypes.UUID) error
	DeleteProject(ctx context.Context, projectId sharedTypes.UUID) error
	GetReadStreamForProjectFile(ctx context.Context, projectId sharedTypes.UUID, fileId sharedTypes.UUID) (int64, io.ReadSeekCloser, error)
	GetRedirectURLForGETOnProjectFile(ctx context.Context, projectId sharedTypes.UUID, fileId sharedTypes.UUID) (*url.URL, error)
	SendStreamForProjectFile(ctx context.Context, projectId sharedTypes.UUID, fileId sharedTypes.UUID, reader io.Reader, size int64) error
}

func New(options objectStorage.Options) (Manager, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}

	b, err := objectStorage.FromOptions(options)
	if err != nil {
		return nil, err
	}
	return &manager{b: b}, nil
}

type manager struct {
	b objectStorage.Backend
}

func getProjectPrefix(projectId sharedTypes.UUID) string {
	return projectId.String() + "/"
}

func getProjectFileKey(projectId, fileId sharedTypes.UUID) string {
	return projectId.Concat('/', fileId)
}

func (m *manager) GetReadStreamForProjectFile(ctx context.Context, projectId sharedTypes.UUID, fileId sharedTypes.UUID) (int64, io.ReadSeekCloser, error) {
	return m.b.GetReadStream(ctx, getProjectFileKey(projectId, fileId))
}

func (m *manager) GetRedirectURLForGETOnProjectFile(ctx context.Context, projectId sharedTypes.UUID, fileId sharedTypes.UUID) (*url.URL, error) {
	return m.b.GetRedirectURLForGET(ctx, getProjectFileKey(projectId, fileId))
}

func (m *manager) CopyProjectFile(ctx context.Context, srcProjectId sharedTypes.UUID, srcFileId sharedTypes.UUID, destProjectId sharedTypes.UUID, destFileId sharedTypes.UUID) error {
	return m.b.CopyObject(
		ctx,
		getProjectFileKey(srcProjectId, srcFileId),
		getProjectFileKey(destProjectId, destFileId),
	)
}

func (m *manager) DeleteProjectFile(ctx context.Context, projectId sharedTypes.UUID, fileId sharedTypes.UUID) error {
	return m.b.DeleteObject(ctx, getProjectFileKey(projectId, fileId))
}

func (m *manager) DeleteProject(ctx context.Context, projectId sharedTypes.UUID) error {
	return m.b.DeletePrefix(ctx, getProjectPrefix(projectId))
}

func (m *manager) SendStreamForProjectFile(ctx context.Context, projectId sharedTypes.UUID, fileId sharedTypes.UUID, reader io.Reader, size int64) error {
	return m.b.SendFromStream(
		ctx,
		getProjectFileKey(projectId, fileId),
		reader,
		size,
	)
}
