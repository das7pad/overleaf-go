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

package tag

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/models/tag"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	AddProjectToTag(ctx context.Context, request *types.AddProjectToTagRequest) error
	CreateTag(ctx context.Context, request *types.CreateTagRequest, response *types.CreateTagResponse) error
	DeleteTag(ctx context.Context, request *types.DeleteTagRequest) error
	RemoveProjectFromTag(ctx context.Context, request *types.RemoveProjectToTagRequest) error
	RenameTag(ctx context.Context, request *types.RenameTagRequest) error
}

func New(tm tag.Manager) Manager {
	return &manager{tm: tm}
}

type manager struct {
	tm tag.Manager
}

func (m *manager) AddProjectToTag(ctx context.Context, r *types.AddProjectToTagRequest) error {
	if err := r.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	return m.tm.AddProject(ctx, r.Session.User.Id, r.TagId, r.ProjectId)
}

func (m *manager) CreateTag(ctx context.Context, r *types.CreateTagRequest, response *types.CreateTagResponse) error {
	if err := r.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	t, err := m.tm.EnsureExists(ctx, r.Session.User.Id, r.Name)
	if err != nil {
		return err
	}
	*response = *t
	return nil
}

func (m *manager) DeleteTag(ctx context.Context, r *types.DeleteTagRequest) error {
	if err := r.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	return m.tm.Delete(ctx, r.Session.User.Id, r.TagId)
}

func (m *manager) RemoveProjectFromTag(ctx context.Context, r *types.RemoveProjectToTagRequest) error {
	if err := r.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	return m.tm.RemoveProject(ctx, r.Session.User.Id, r.TagId, r.ProjectId)
}

func (m *manager) RenameTag(ctx context.Context, r *types.RenameTagRequest) error {
	if err := r.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	return m.tm.Rename(ctx, r.Session.User.Id, r.TagId, r.Name)
}
