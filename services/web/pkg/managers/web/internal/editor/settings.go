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

package editor

import (
	"context"
	"time"

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
		return errors.Tag(err, "update compiler")
	}
	go m.notifyEditor(
		request.ProjectId, sharedTypes.CompilerUpdated, request.Compiler,
	)
	return nil
}

func (m *manager) SetImageName(ctx context.Context, request *types.SetImageNameRequest) error {
	if err := request.ImageName.Validate(); err != nil {
		return err
	}
	err := request.ImageName.CheckIsAllowed(m.allowedImageNames)
	if err != nil {
		return err
	}
	err = m.pm.SetImageName(
		ctx, request.ProjectId, request.UserId, request.ImageName,
	)
	if err != nil {
		return errors.Tag(err, "update compiler")
	}
	go m.notifyEditor(
		request.ProjectId, sharedTypes.ImageNameUpdated, request.ImageName,
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
		return errors.Tag(err, "update spell check language")
	}
	go m.notifyEditor(
		request.ProjectId,
		sharedTypes.SpellCheckLanguageUpdated,
		request.SpellCheckLanguage,
	)
	return nil
}

func (m *manager) SetRootDocId(ctx context.Context, r *types.SetRootDocIdRequest) error {
	if r.RootDocId.IsZero() {
		return &errors.ValidationError{Msg: "missing rootDocId"}
	}
	err := m.pm.SetRootDoc(ctx, r.ProjectId, r.UserId, r.RootDocId)
	if err != nil {
		return errors.Tag(err, "update rootDoc")
	}
	go m.notifyEditor(r.ProjectId, sharedTypes.RootDocUpdated, r.RootDocId)
	return nil
}

type projectEditableUpdatedDetails struct {
	project.ContentLockedAtField
	project.EditableField
}

func (m *manager) SetContentLocked(ctx context.Context, r *types.SetContentLockedRequest) error {
	if err := m.dum.FlushAndDeleteProject(ctx, r.ProjectId); err != nil {
		return errors.Tag(err, "flush before")
	}
	d := projectEditableUpdatedDetails{}
	if r.ContentLocked {
		now := time.Now()
		d.ContentLockedAt = &now
	}
	editable, err := m.pm.SetContentLockedAt(
		ctx, r.ProjectId, r.UserId, d.ContentLockedAt,
	)
	if err != nil {
		return errors.Tag(err, "update content locked")
	}
	d.Editable = editable
	m.notifyEditor(r.ProjectId, sharedTypes.ProjectEditableUpdated, d)
	if err = m.dum.FlushAndDeleteProject(ctx, r.ProjectId); err != nil {
		return errors.Tag(err, "flush after")
	}
	return nil
}
