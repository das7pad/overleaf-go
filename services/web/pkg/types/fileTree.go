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

package types

import (
	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type AddDocRequest struct {
	ProjectId      edgedb.UUID          `json:"-"`
	UserId         edgedb.UUID          `json:"-"`
	Name           sharedTypes.Filename `json:"name"`
	ParentFolderId edgedb.UUID          `json:"parent_folder_id"`
}

type AddDocResponse = project.Doc

type AddFolderRequest struct {
	ProjectId      edgedb.UUID          `json:"-"`
	UserId         edgedb.UUID          `edgedb:"-"`
	Name           sharedTypes.Filename `json:"name"`
	ParentFolderId edgedb.UUID          `json:"parent_folder_id"`
}

type AddFolderResponse = project.Folder

type DeleteDocRequest struct {
	ProjectId edgedb.UUID `json:"-"`
	DocId     edgedb.UUID `json:"-"`
}

type DeleteFileRequest struct {
	ProjectId edgedb.UUID `json:"-"`
	FileId    edgedb.UUID `json:"-"`
}

type DeleteFolderRequest struct {
	ProjectId edgedb.UUID `json:"-"`
	FolderId  edgedb.UUID `json:"-"`
}

type MoveDocRequest struct {
	ProjectId      edgedb.UUID `json:"-"`
	DocId          edgedb.UUID `json:"-"`
	TargetFolderId edgedb.UUID `json:"folder_id"`
}

type MoveFileRequest struct {
	ProjectId      edgedb.UUID `json:"-"`
	FileId         edgedb.UUID `json:"-"`
	TargetFolderId edgedb.UUID `json:"folder_id"`
}

type MoveFolderRequest struct {
	ProjectId      edgedb.UUID `json:"-"`
	FolderId       edgedb.UUID `json:"-"`
	TargetFolderId edgedb.UUID `json:"folder_id"`
}

type RenameDocRequest struct {
	ProjectId edgedb.UUID          `json:"-"`
	DocId     edgedb.UUID          `json:"-"`
	Name      sharedTypes.Filename `json:"name"`
}

type RenameFileRequest struct {
	ProjectId edgedb.UUID          `json:"-"`
	FileId    edgedb.UUID          `json:"-"`
	Name      sharedTypes.Filename `json:"name"`
}

type RenameFolderRequest struct {
	ProjectId edgedb.UUID          `json:"-"`
	FolderId  edgedb.UUID          `json:"-"`
	Name      sharedTypes.Filename `json:"name"`
}

type RestoreDeletedDocRequest struct {
	ProjectId edgedb.UUID          `json:"-"`
	DocId     edgedb.UUID          `json:"-"`
	Name      sharedTypes.Filename `json:"name"`
}

type RestoreDeletedDocResponse struct {
	DocId edgedb.UUID `json:"doc_id"`
}
