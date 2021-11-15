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
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type AddDocRequest struct {
	ProjectId      primitive.ObjectID   `json:"-"`
	Name           sharedTypes.Filename `json:"name"`
	ParentFolderId primitive.ObjectID   `json:"parent_folder_id"`
}

type AddDocResponse = project.Doc

type AddFolderRequest struct {
	ProjectId      primitive.ObjectID   `json:"-"`
	Name           sharedTypes.Filename `json:"name"`
	ParentFolderId primitive.ObjectID   `json:"parent_folder_id"`
}

type AddFolderResponse = project.Folder

type DeleteDocRequest struct {
	ProjectId primitive.ObjectID `json:"-"`
	DocId     primitive.ObjectID `json:"-"`
}

type DeleteFileRequest struct {
	ProjectId primitive.ObjectID `json:"-"`
	FileId    primitive.ObjectID `json:"-"`
}

type DeleteFolderRequest struct {
	ProjectId primitive.ObjectID `json:"-"`
	FolderId  primitive.ObjectID `json:"-"`
}

type MoveDocRequest struct {
	ProjectId      primitive.ObjectID `json:"-"`
	DocId          primitive.ObjectID `json:"-"`
	TargetFolderId primitive.ObjectID `json:"folder_id"`
}

type MoveFileRequest struct {
	ProjectId      primitive.ObjectID `json:"-"`
	FileId         primitive.ObjectID `json:"-"`
	TargetFolderId primitive.ObjectID `json:"folder_id"`
}

type MoveFolderRequest struct {
	ProjectId      primitive.ObjectID `json:"-"`
	FolderId       primitive.ObjectID `json:"-"`
	TargetFolderId primitive.ObjectID `json:"folder_id"`
}

type RenameDocRequest struct {
	ProjectId primitive.ObjectID   `json:"-"`
	DocId     primitive.ObjectID   `json:"-"`
	Name      sharedTypes.Filename `json:"name"`
}

type RenameFileRequest struct {
	ProjectId primitive.ObjectID   `json:"-"`
	FileId    primitive.ObjectID   `json:"-"`
	Name      sharedTypes.Filename `json:"name"`
}

type RenameFolderRequest struct {
	ProjectId primitive.ObjectID   `json:"-"`
	FolderId  primitive.ObjectID   `json:"-"`
	Name      sharedTypes.Filename `json:"name"`
}
