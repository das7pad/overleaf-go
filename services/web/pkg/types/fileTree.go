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

package types

import (
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type AddDocRequest struct {
	ProjectId      sharedTypes.UUID     `json:"-"`
	UserId         sharedTypes.UUID     `json:"-"`
	Name           sharedTypes.Filename `json:"name"`
	ParentFolderId sharedTypes.UUID     `json:"parent_folder_id"`
}

type AddDocResponse = project.Doc

type AddFolderRequest struct {
	ProjectId      sharedTypes.UUID `json:"-"`
	UserId         sharedTypes.UUID
	Name           sharedTypes.Filename `json:"name"`
	ParentFolderId sharedTypes.UUID     `json:"parent_folder_id"`
}

type AddFolderResponse = project.Folder

type DeleteDocRequest struct {
	ProjectId sharedTypes.UUID `json:"-"`
	UserId    sharedTypes.UUID `json:"-"`
	DocId     sharedTypes.UUID `json:"-"`
}

type DeleteFileRequest struct {
	ProjectId sharedTypes.UUID `json:"-"`
	UserId    sharedTypes.UUID `json:"-"`
	FileId    sharedTypes.UUID `json:"-"`
}

type DeleteFolderRequest struct {
	ProjectId sharedTypes.UUID `json:"-"`
	UserId    sharedTypes.UUID `json:"-"`
	FolderId  sharedTypes.UUID `json:"-"`
}

type MoveDocRequest struct {
	ProjectId      sharedTypes.UUID `json:"-"`
	UserId         sharedTypes.UUID `json:"-"`
	DocId          sharedTypes.UUID `json:"-"`
	TargetFolderId sharedTypes.UUID `json:"folder_id"`
}

type MoveFileRequest struct {
	ProjectId      sharedTypes.UUID `json:"-"`
	UserId         sharedTypes.UUID `json:"-"`
	FileId         sharedTypes.UUID `json:"-"`
	TargetFolderId sharedTypes.UUID `json:"folder_id"`
}

type MoveFolderRequest struct {
	ProjectId      sharedTypes.UUID `json:"-"`
	UserId         sharedTypes.UUID `json:"-"`
	FolderId       sharedTypes.UUID `json:"-"`
	TargetFolderId sharedTypes.UUID `json:"folder_id"`
}

type RenameDocRequest struct {
	ProjectId sharedTypes.UUID     `json:"-"`
	UserId    sharedTypes.UUID     `json:"-"`
	DocId     sharedTypes.UUID     `json:"-"`
	Name      sharedTypes.Filename `json:"name"`
}

type RenameFileRequest struct {
	ProjectId sharedTypes.UUID     `json:"-"`
	UserId    sharedTypes.UUID     `json:"-"`
	FileId    sharedTypes.UUID     `json:"-"`
	Name      sharedTypes.Filename `json:"name"`
}

type RenameFolderRequest struct {
	ProjectId sharedTypes.UUID     `json:"-"`
	UserId    sharedTypes.UUID     `json:"-"`
	FolderId  sharedTypes.UUID     `json:"-"`
	Name      sharedTypes.Filename `json:"name"`
}

type RestoreDeletedDocRequest struct {
	ProjectId sharedTypes.UUID     `json:"-"`
	UserId    sharedTypes.UUID     `json:"-"`
	DocId     sharedTypes.UUID     `json:"-"`
	Name      sharedTypes.Filename `json:"name"`
}

type RestoreDeletedDocResponse struct {
	DocId sharedTypes.UUID `json:"doc_id"`
}
