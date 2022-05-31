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

package editor

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) UpdateEditorConfig(ctx context.Context, request *types.UpdateEditorConfigRequest) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	if err := request.EditorConfig.Validate(); err != nil {
		return err
	}
	userId := request.Session.User.Id
	return m.um.UpdateEditorConfig(ctx, userId, request.EditorConfig)
}

func (m *manager) SetCompiler(ctx context.Context, request *types.SetCompilerRequest) error {
	if err := request.Compiler.Validate(); err != nil {
		return err
	}
	err := m.pm.SetCompiler(
		ctx, request.ProjectId, request.UserId, request.Compiler,
	)
	if err != nil {
		return errors.Tag(err, "cannot update compiler")
	}
	go m.notifyEditor(
		request.ProjectId, "compilerUpdated", request.Compiler,
	)
	return nil
}

func (m *manager) SetImageName(ctx context.Context, request *types.SetImageNameRequest) error {
	if err := request.ImageName.Validate(); err != nil {
		return err
	}
	err := request.ImageName.CheckIsAllowed(m.options.AllowedImages)
	if err != nil {
		return err
	}
	err = m.pm.SetImageName(
		ctx, request.ProjectId, request.UserId, request.ImageName,
	)
	if err != nil {
		return errors.Tag(err, "cannot update compiler")
	}
	go m.notifyEditor(
		request.ProjectId, "imageNameUpdated", request.ImageName,
	)
	return nil
}

func (m *manager) SetSpellCheckLanguage(ctx context.Context, request *types.SetSpellCheckLanguageRequest) error {
	if request.SpellCheckLanguage == "" {
		// disable spell checking
	} else if err := request.SpellCheckLanguage.Validate(); err != nil {
		return err
	}
	err := m.pm.SetSpellCheckLanguage(
		ctx, request.ProjectId, request.UserId, request.SpellCheckLanguage,
	)
	if err != nil {
		return errors.Tag(err, "cannot update compiler")
	}
	go m.notifyEditor(
		request.ProjectId,
		"spellCheckLanguageUpdated",
		request.SpellCheckLanguage,
	)
	return nil
}

func (m *manager) SetRootDocId(ctx context.Context, r *types.SetRootDocIdRequest) error {
	if r.RootDocId == (sharedTypes.UUID{}) {
		return &errors.ValidationError{Msg: "missing rootDocId"}
	}
	err := m.pm.SetRootDoc(ctx, r.ProjectId, r.UserId, r.RootDocId)
	if err != nil {
		return errors.Tag(err, "cannot update rootDoc")
	}
	go m.notifyEditor(r.ProjectId, "rootDocUpdated", r.RootDocId)
	return nil
}

type publicAccessLevelChangedBody struct {
	NewAccessLevel project.PublicAccessLevel `json:"newAccessLevel"`
}

type tokensChangedBody struct {
	Tokens *project.Tokens `json:"tokens"`
}

func (m *manager) SetPublicAccessLevel(ctx context.Context, request *types.SetPublicAccessLevelRequest) error {
	if err := request.PublicAccessLevel.Validate(); err != nil {
		return err
	}

	if request.PublicAccessLevel == project.TokenBasedAccess {
		tokens, err := m.pm.PopulateTokens(
			ctx, request.ProjectId, request.UserId,
		)
		if err != nil {
			return errors.Tag(err, "cannot populate tokens")
		}
		if tokens != nil {
			go m.notifyEditor(
				request.ProjectId,
				"project:tokens:changed",
				tokensChangedBody{Tokens: tokens},
			)
		}
	}

	err := m.pm.SetPublicAccessLevel(ctx, request.ProjectId, request.UserId, request.PublicAccessLevel)
	if err != nil {
		return errors.Tag(err, "cannot update PublicAccessLevel")
	}

	go m.notifyEditor(
		request.ProjectId,
		"project:publicAccessLevel:changed",
		publicAccessLevelChangedBody{
			NewAccessLevel: request.PublicAccessLevel,
		},
	)
	return nil
}
