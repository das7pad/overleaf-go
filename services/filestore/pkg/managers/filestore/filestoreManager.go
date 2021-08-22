// Golang port of the Overleaf filestore service
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

package filestore

import (
	"context"
	"fmt"
	"io"
	"net/url"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/services/filestore/pkg/backend"
	"github.com/das7pad/overleaf-go/services/filestore/pkg/types"
)

type Manager interface {
	GetReadStreamForProjectFile(
		ctx context.Context,
		projectId primitive.ObjectID,
		fileId primitive.ObjectID,
		options backend.GetOptions,
	) (io.Reader, error)

	GetRedirectURLForGETOnProjectFile(
		ctx context.Context,
		projectId primitive.ObjectID,
		fileId primitive.ObjectID,
	) (*url.URL, error)

	GetRedirectURLForHEADOnProjectFile(
		ctx context.Context,
		projectId primitive.ObjectID,
		fileId primitive.ObjectID,
	) (*url.URL, error)

	GetRedirectURLForPOSTOnProjectFile(
		ctx context.Context,
		projectId primitive.ObjectID,
		fileId primitive.ObjectID,
	) (*url.URL, backend.FormData, error)

	GetRedirectURLForPUTOnProjectFile(
		ctx context.Context,
		projectId primitive.ObjectID,
		fileId primitive.ObjectID,
	) (*url.URL, error)

	GetSizeOfProjectFile(
		ctx context.Context,
		projectId primitive.ObjectID,
		fileId primitive.ObjectID,
	) (int64, error)

	GetSizeOfProject(
		ctx context.Context,
		projectId primitive.ObjectID,
	) (int64, error)

	CopyProjectFile(
		ctx context.Context,
		srcProjectId primitive.ObjectID,
		srcFileId primitive.ObjectID,
		destProjectId primitive.ObjectID,
		destFileId primitive.ObjectID,
	) error

	DeleteProjectFile(
		ctx context.Context,
		projectId primitive.ObjectID,
		fileId primitive.ObjectID,
	) error

	DeleteProject(
		ctx context.Context,
		projectId primitive.ObjectID,
	) error

	SendStreamForProjectFile(
		ctx context.Context,
		projectId primitive.ObjectID,
		fileId primitive.ObjectID,
		reader io.Reader,
		options backend.SendOptions,
	) error
}

func New(options *types.Options) (Manager, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}

	b, err := backend.FromOptions(options.BackendOptions)
	if err != nil {
		return nil, err
	}
	return &manager{
		b:       b,
		buckets: options.Buckets,
	}, nil
}

type manager struct {
	b       backend.Backend
	buckets types.Buckets
}

func getProjectPrefix(projectId primitive.ObjectID) string {
	return fmt.Sprintf("%s/", projectId.Hex())
}

func getProjectFileKey(projectId, fileId primitive.ObjectID) string {
	return fmt.Sprintf("%s/%s", projectId.Hex(), fileId.Hex())
}

func (m *manager) GetReadStreamForProjectFile(ctx context.Context, projectId primitive.ObjectID, fileId primitive.ObjectID, options backend.GetOptions) (io.Reader, error) {
	return m.b.GetReadStream(
		ctx,
		m.buckets.UserFiles,
		getProjectFileKey(projectId, fileId),
		options,
	)
}

func (m *manager) GetRedirectURLForGETOnProjectFile(ctx context.Context, projectId primitive.ObjectID, fileId primitive.ObjectID) (*url.URL, error) {
	return m.b.GetRedirectURLForGET(
		ctx,
		m.buckets.UserFiles,
		getProjectFileKey(projectId, fileId),
	)
}

func (m *manager) GetRedirectURLForHEADOnProjectFile(ctx context.Context, projectId primitive.ObjectID, fileId primitive.ObjectID) (*url.URL, error) {
	return m.b.GetRedirectURLForHEAD(
		ctx,
		m.buckets.UserFiles,
		getProjectFileKey(projectId, fileId),
	)
}

func (m *manager) GetRedirectURLForPOSTOnProjectFile(ctx context.Context, projectId primitive.ObjectID, fileId primitive.ObjectID) (*url.URL, backend.FormData, error) {
	return m.b.GetRedirectURLForPOST(
		ctx,
		m.buckets.UserFiles,
		getProjectFileKey(projectId, fileId),
	)
}

func (m *manager) GetRedirectURLForPUTOnProjectFile(ctx context.Context, projectId primitive.ObjectID, fileId primitive.ObjectID) (*url.URL, error) {
	return m.b.GetRedirectURLForPUT(
		ctx,
		m.buckets.UserFiles,
		getProjectFileKey(projectId, fileId),
	)
}

func (m *manager) GetSizeOfProjectFile(ctx context.Context, projectId primitive.ObjectID, fileId primitive.ObjectID) (int64, error) {
	return m.b.GetObjectSize(
		ctx,
		m.buckets.UserFiles,
		getProjectFileKey(projectId, fileId),
	)
}

func (m *manager) GetSizeOfProject(ctx context.Context, projectId primitive.ObjectID) (int64, error) {
	return m.b.GetDirectorySize(
		ctx,
		m.buckets.UserFiles,
		getProjectPrefix(projectId),
	)
}

func (m *manager) CopyProjectFile(ctx context.Context, srcProjectId primitive.ObjectID, srcFileId primitive.ObjectID, destProjectId primitive.ObjectID, destFileId primitive.ObjectID) error {
	return m.b.CopyObject(
		ctx,
		m.buckets.UserFiles,
		getProjectFileKey(srcProjectId, srcFileId),
		getProjectFileKey(destProjectId, destFileId),
	)
}

func (m *manager) DeleteProjectFile(ctx context.Context, projectId primitive.ObjectID, fileId primitive.ObjectID) error {
	return m.b.DeleteObject(
		ctx,
		m.buckets.UserFiles,
		getProjectFileKey(projectId, fileId),
	)
}

func (m *manager) DeleteProject(ctx context.Context, projectId primitive.ObjectID) error {
	return m.b.DeletePrefix(
		ctx,
		m.buckets.UserFiles,
		getProjectPrefix(projectId),
	)
}

func (m *manager) SendStreamForProjectFile(ctx context.Context, projectId primitive.ObjectID, fileId primitive.ObjectID, reader io.Reader, options backend.SendOptions) error {
	return m.b.SendFromStream(
		ctx,
		m.buckets.UserFiles,
		getProjectFileKey(projectId, fileId),
		reader,
		options,
	)
}
