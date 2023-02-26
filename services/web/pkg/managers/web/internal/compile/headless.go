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

package compile

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) CompileHeadLess(ctx context.Context, r *types.CompileProjectHeadlessRequest, response *types.CompileProjectResponse) error {
	d, err := m.pm.GetLoadEditorDetails(ctx, r.ProjectId, r.UserId, "")
	if err != nil {
		return err
	}
	p := d.Project
	if _, err = p.GetPrivilegeLevelAuthenticated(); err != nil {
		return err
	}

	return m.Compile(ctx, &types.CompileProjectRequest{
		SignedCompileProjectRequestOptions: sharedTypes.SignedCompileProjectRequestOptions{
			CompileGroup: p.OwnerFeatures.CompileGroup,
			ProjectId:    r.ProjectId,
			UserId:       r.UserId,
			Timeout:      p.OwnerFeatures.CompileTimeout,
		},
		AutoCompile:                false,
		CheckMode:                  clsiTypes.SilentCheck,
		Compiler:                   p.Compiler,
		Draft:                      false,
		ImageName:                  p.ImageName,
		IncrementalCompilesEnabled: true,
		RootDocId:                  p.RootDoc.Id,
		RootDocPath:                p.RootDoc.Path,
		SyncState:                  clsiTypes.SyncState(p.Version.String()),
	}, response)
}
