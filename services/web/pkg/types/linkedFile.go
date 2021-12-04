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

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type CreateLinkedFileProviderParameter struct {
	ClsiServerId         ClsiServerId         `json:"clsiServerId"`
	BuildId              clsiTypes.BuildId    `json:"build_id"`
	SourceProjectId      primitive.ObjectID   `json:"source_project_id"`
	SourceEntityPath     sharedTypes.PathName `json:"source_entity_path"`
	SourceOutputFilePath sharedTypes.PathName `json:"source_output_file_path"`
	URL                  *sharedTypes.URL     `json:"url"`
}

type CreateLinkedFileRequest struct {
	UserId         primitive.ObjectID                `json:"-"`
	ProjectId      primitive.ObjectID                `json:"-"`
	ParentFolderId primitive.ObjectID                `json:"parent_folder_id"`
	Name           sharedTypes.Filename              `json:"name"`
	Provider       project.LinkedFileProvider        `json:"provider"`
	Parameter      CreateLinkedFileProviderParameter `json:"data"`
}

type RefreshLinkedFileRequest struct {
	UserId    primitive.ObjectID `json:"-"`
	ProjectId primitive.ObjectID `json:"-"`
	FileId    primitive.ObjectID `json:"-"`

	ParentFolderId primitive.ObjectID `json:"-"`
	File           *project.FileRef   `json:"-"`
}

func (r *CreateLinkedFileRequest) LinkedFileData() *project.LinkedFileData {
	return &project.LinkedFileData{
		Provider:             r.Provider,
		SourceProjectId:      r.Parameter.SourceProjectId.Hex(),
		SourceEntityPath:     r.Parameter.SourceEntityPath.String(),
		SourceOutputFilePath: r.Parameter.SourceOutputFilePath.String(),
		URL:                  r.Parameter.URL,
	}
}

func (r *CreateLinkedFileRequest) Validate() error {
	if err := r.Provider.Validate(); err != nil {
		return err
	}
	if err := r.Name.Validate(); err != nil {
		return err
	}

	switch r.Provider {
	case project.LinkedFileProviderURL:
		if r.Parameter.URL == nil {
			return &errors.ValidationError{Msg: "missing url"}
		}
		if err := r.Parameter.URL.Validate(); err != nil {
			return err
		}
		return nil
	case project.LinkedFileProviderProjectFile:
		if r.Parameter.SourceProjectId.IsZero() {
			return &errors.ValidationError{Msg: "missing source_project_id"}
		}
		if err := r.Parameter.SourceEntityPath.Validate(); err != nil {
			return err
		}
		return nil
	case project.LinkedFileProviderProjectOutputFile:
		if r.Parameter.SourceProjectId.IsZero() {
			return &errors.ValidationError{Msg: "missing source_project_id"}
		}
		if err := r.Parameter.SourceOutputFilePath.Validate(); err != nil {
			return err
		}
		if err := r.Parameter.BuildId.Validate(); err != nil {
			return err
		}
		return nil
	default:
		return &errors.ValidationError{Msg: "unknown provider"}
	}
}
