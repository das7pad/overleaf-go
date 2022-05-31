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

package filestore

import (
	"context"
	"fmt"
	"io"
	"net/url"

	"github.com/das7pad/overleaf-go/pkg/objectStorage"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/filestore/pkg/types"
)

type Manager interface {
	GetReadStreamForProjectFile(
		ctx context.Context,
		projectId sharedTypes.UUID,
		fileId sharedTypes.UUID,
		options objectStorage.GetOptions,
	) (int64, io.ReadCloser, error)

	GetRedirectURLForGETOnProjectFile(
		ctx context.Context,
		projectId sharedTypes.UUID,
		fileId sharedTypes.UUID,
	) (*url.URL, error)

	GetRedirectURLForHEADOnProjectFile(
		ctx context.Context,
		projectId sharedTypes.UUID,
		fileId sharedTypes.UUID,
	) (*url.URL, error)

	GetRedirectURLForPOSTOnProjectFile(
		ctx context.Context,
		projectId sharedTypes.UUID,
		fileId sharedTypes.UUID,
	) (*url.URL, objectStorage.FormData, error)

	GetRedirectURLForPUTOnProjectFile(
		ctx context.Context,
		projectId sharedTypes.UUID,
		fileId sharedTypes.UUID,
	) (*url.URL, error)

	GetSizeOfProjectFile(
		ctx context.Context,
		projectId sharedTypes.UUID,
		fileId sharedTypes.UUID,
	) (int64, error)

	GetSizeOfProject(
		ctx context.Context,
		projectId sharedTypes.UUID,
	) (int64, error)

	CopyProjectFile(
		ctx context.Context,
		srcProjectId sharedTypes.UUID,
		srcFileId sharedTypes.UUID,
		destProjectId sharedTypes.UUID,
		destFileId sharedTypes.UUID,
	) error

	DeleteProjectFile(
		ctx context.Context,
		projectId sharedTypes.UUID,
		fileId sharedTypes.UUID,
	) error

	DeleteProject(
		ctx context.Context,
		projectId sharedTypes.UUID,
	) error

	SendProjectFileFromFS(ctx context.Context, projectId sharedTypes.UUID, fileId sharedTypes.UUID, path sharedTypes.PathName, options objectStorage.SendOptions) error

	SendStreamForProjectFile(
		ctx context.Context,
		projectId sharedTypes.UUID,
		fileId sharedTypes.UUID,
		reader io.Reader,
		options objectStorage.SendOptions,
	) error
}

func New(options *types.Options) (Manager, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}

	b, err := objectStorage.FromOptions(options.BackendOptions)
	if err != nil {
		return nil, err
	}
	return &manager{
		b:          b,
		buckets:    options.Buckets,
		uploadBase: options.UploadBase,
	}, nil
}

type manager struct {
	b          objectStorage.Backend
	buckets    types.Buckets
	uploadBase sharedTypes.DirName
}

func getProjectPrefix(projectId sharedTypes.UUID) string {
	return fmt.Sprintf("%s/", projectId.String())
}

func getProjectFileKey(projectId, fileId sharedTypes.UUID) string {
	return fmt.Sprintf("%s/%s", projectId.String(), fileId.String())
}

func (m *manager) GetReadStreamForProjectFile(ctx context.Context, projectId sharedTypes.UUID, fileId sharedTypes.UUID, options objectStorage.GetOptions) (int64, io.ReadCloser, error) {
	return m.b.GetReadStream(
		ctx,
		m.buckets.UserFiles,
		getProjectFileKey(projectId, fileId),
		options,
	)
}

func (m *manager) GetRedirectURLForGETOnProjectFile(ctx context.Context, projectId sharedTypes.UUID, fileId sharedTypes.UUID) (*url.URL, error) {
	return m.b.GetRedirectURLForGET(
		ctx,
		m.buckets.UserFiles,
		getProjectFileKey(projectId, fileId),
	)
}

func (m *manager) GetRedirectURLForHEADOnProjectFile(ctx context.Context, projectId sharedTypes.UUID, fileId sharedTypes.UUID) (*url.URL, error) {
	return m.b.GetRedirectURLForHEAD(
		ctx,
		m.buckets.UserFiles,
		getProjectFileKey(projectId, fileId),
	)
}

func (m *manager) GetRedirectURLForPOSTOnProjectFile(ctx context.Context, projectId sharedTypes.UUID, fileId sharedTypes.UUID) (*url.URL, objectStorage.FormData, error) {
	return m.b.GetRedirectURLForPOST(
		ctx,
		m.buckets.UserFiles,
		getProjectFileKey(projectId, fileId),
	)
}

func (m *manager) GetRedirectURLForPUTOnProjectFile(ctx context.Context, projectId sharedTypes.UUID, fileId sharedTypes.UUID) (*url.URL, error) {
	return m.b.GetRedirectURLForPUT(
		ctx,
		m.buckets.UserFiles,
		getProjectFileKey(projectId, fileId),
	)
}

func (m *manager) GetSizeOfProjectFile(ctx context.Context, projectId sharedTypes.UUID, fileId sharedTypes.UUID) (int64, error) {
	return m.b.GetObjectSize(
		ctx,
		m.buckets.UserFiles,
		getProjectFileKey(projectId, fileId),
	)
}

func (m *manager) GetSizeOfProject(ctx context.Context, projectId sharedTypes.UUID) (int64, error) {
	return m.b.GetDirectorySize(
		ctx,
		m.buckets.UserFiles,
		getProjectPrefix(projectId),
	)
}

func (m *manager) CopyProjectFile(ctx context.Context, srcProjectId sharedTypes.UUID, srcFileId sharedTypes.UUID, destProjectId sharedTypes.UUID, destFileId sharedTypes.UUID) error {
	return m.b.CopyObject(
		ctx,
		m.buckets.UserFiles,
		getProjectFileKey(srcProjectId, srcFileId),
		getProjectFileKey(destProjectId, destFileId),
	)
}

func (m *manager) DeleteProjectFile(ctx context.Context, projectId sharedTypes.UUID, fileId sharedTypes.UUID) error {
	return m.b.DeleteObject(
		ctx,
		m.buckets.UserFiles,
		getProjectFileKey(projectId, fileId),
	)
}

func (m *manager) DeleteProject(ctx context.Context, projectId sharedTypes.UUID) error {
	return m.b.DeletePrefix(
		ctx,
		m.buckets.UserFiles,
		getProjectPrefix(projectId),
	)
}

func (m *manager) SendStreamForProjectFile(ctx context.Context, projectId sharedTypes.UUID, fileId sharedTypes.UUID, reader io.Reader, options objectStorage.SendOptions) error {
	return m.b.SendFromStream(
		ctx,
		m.buckets.UserFiles,
		getProjectFileKey(projectId, fileId),
		reader,
		options,
	)
}

func (m *manager) SendProjectFileFromFS(ctx context.Context, projectId sharedTypes.UUID, fileId sharedTypes.UUID, path sharedTypes.PathName, options objectStorage.SendOptions) error {
	if err := path.Validate(); err != nil {
		return err
	}
	return m.b.SendFromFile(
		ctx,
		m.buckets.UserFiles,
		getProjectFileKey(projectId, fileId),
		string(m.uploadBase.JoinPath(path)),
		options,
	)
}
