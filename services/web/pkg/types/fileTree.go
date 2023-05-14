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

package types

import (
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type AddDocRequest struct {
	WithProjectIdAndUserId
	Name           sharedTypes.Filename `json:"name"`
	ParentFolderId sharedTypes.UUID     `json:"parent_folder_id"`
	ClientId       sharedTypes.PublicId `json:"clientId"`
}

func (r *AddDocRequest) Validate() error {
	if err := r.Name.Validate(); err != nil {
		return err
	}
	if err := r.ClientId.Validate(); err != nil {
		return err
	}
	return nil
}

type AddDocResponse = project.Doc

type AddFolderRequest struct {
	WithProjectIdAndUserId
	Name           sharedTypes.Filename `json:"name"`
	ParentFolderId sharedTypes.UUID     `json:"parent_folder_id"`
}

type AddFolderResponse = project.Folder

type DeleteDocRequest struct {
	WithProjectIdAndUserId
	DocId sharedTypes.UUID `json:"-"`
}

type DeleteFileRequest struct {
	WithProjectIdAndUserId
	FileId sharedTypes.UUID `json:"-"`
}

type DeleteFolderRequest struct {
	WithProjectIdAndUserId
	FolderId sharedTypes.UUID `json:"-"`
}

type MoveDocRequest struct {
	WithProjectIdAndUserId
	DocId          sharedTypes.UUID `json:"-"`
	TargetFolderId sharedTypes.UUID `json:"folder_id"`
}

type MoveFileRequest struct {
	WithProjectIdAndUserId
	FileId         sharedTypes.UUID `json:"-"`
	TargetFolderId sharedTypes.UUID `json:"folder_id"`
}

type MoveFolderRequest struct {
	WithProjectIdAndUserId
	FolderId       sharedTypes.UUID `json:"-"`
	TargetFolderId sharedTypes.UUID `json:"folder_id"`
}

type RenameDocRequest struct {
	WithProjectIdAndUserId
	DocId sharedTypes.UUID     `json:"-"`
	Name  sharedTypes.Filename `json:"name"`
}

type RenameFileRequest struct {
	WithProjectIdAndUserId
	FileId sharedTypes.UUID     `json:"-"`
	Name   sharedTypes.Filename `json:"name"`
}

type RenameFolderRequest struct {
	WithProjectIdAndUserId
	FolderId sharedTypes.UUID     `json:"-"`
	Name     sharedTypes.Filename `json:"name"`
}

type RestoreDeletedDocRequest struct {
	WithProjectIdAndUserId
	DocId sharedTypes.UUID     `json:"-"`
	Name  sharedTypes.Filename `json:"name"`
}

type RestoreDeletedDocResponse struct {
	DocId sharedTypes.UUID `json:"doc_id"`
}
