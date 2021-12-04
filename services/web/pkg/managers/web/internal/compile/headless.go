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

package compile

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) CompileHeadLess(ctx context.Context, request *types.CompileProjectHeadlessRequest, response *types.CompileProjectResponse) error {
	p, err := m.pm.GetLoadEditorDetails(ctx, request.ProjectId, request.UserId)
	if err != nil {
		return err
	}
	if _, err = p.GetPrivilegeLevelAuthenticated(request.UserId); err != nil {
		return err
	}
	t, err := p.GetRootFolder()
	if err != nil {
		return err
	}

	owner := &user.FeaturesField{}
	if err = m.um.GetUser(ctx, p.OwnerRef, owner); err != nil {
		return errors.Tag(err, "cannot get owner features")
	}

	var rootDocPath sharedTypes.PathName
	_ = t.WalkDocs(func(e project.TreeElement, path sharedTypes.PathName) error {
		if e.GetId() == p.RootDocId {
			rootDocPath = path
			return project.AbortWalk
		}
		return nil
	})

	return m.Compile(ctx, &types.CompileProjectRequest{
		SignedCompileProjectRequestOptions: types.SignedCompileProjectRequestOptions{
			CompileGroup: owner.Features.CompileGroup,
			ProjectId:    request.ProjectId,
			UserId:       request.UserId,
			Timeout:      owner.Features.CompileTimeout,
		},
		AutoCompile:                false,
		CheckMode:                  clsiTypes.SilentCheck,
		Compiler:                   p.Compiler,
		Draft:                      false,
		ImageName:                  p.ImageName,
		IncrementalCompilesEnabled: true,
		RootDocId:                  p.RootDocId,
		RootDocPath:                clsiTypes.RootResourcePath(rootDocPath),
		SyncState:                  clsiTypes.SyncState(p.Version.String()),
	}, response)
}
