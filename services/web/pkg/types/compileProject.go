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

	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type SignedCompileProjectRequestOptions struct {
	CompileGroup clsiTypes.CompileGroup `json:"c"`
	ProjectId    primitive.ObjectID     `json:"p"`
	UserId       primitive.ObjectID     `json:"u"`
	Timeout      clsiTypes.Timeout      `json:"t"`
}

type CompileProjectRequest struct {
	SignedCompileProjectRequestOptions `json:"-"`

	AutoCompile                bool                       `json:"autoCompile"`
	CheckMode                  clsiTypes.CheckMode        `json:"checkMode"`
	Compiler                   clsiTypes.Compiler         `json:"compiler"`
	Draft                      clsiTypes.DraftModeFlag    `json:"draft"`
	ImageName                  clsiTypes.ImageName        `json:"imageName"`
	IncrementalCompilesEnabled bool                       `json:"incrementalCompilesEnabled"`
	RootDocId                  primitive.ObjectID         `json:"rootDocId"`
	RootDocPath                clsiTypes.RootResourcePath `json:"rootDocPath"`
	SyncState                  clsiTypes.SyncState        `json:"syncState"`
}

func (r *CompileProjectRequest) Validate() error {
	if err := r.CheckMode.Validate(); err != nil {
		return err
	}
	if err := r.Compiler.Validate(); err != nil {
		return err
	}
	if err := r.Draft.Validate(); err != nil {
		return err
	}
	if err := r.ImageName.Validate(); err != nil {
		return err
	}
	if r.RootDocPath != "" {
		if err := r.RootDocPath.Validate(); err != nil {
			return err
		}
	}
	if err := r.SyncState.Validate(); err != nil {
		return err
	}
	return nil
}

type ClsiServerId string
type PDFDownloadDomain string

type CompileProjectResponse struct {
	clsiTypes.CompileResponse
	ClsiServerId      ClsiServerId           `json:"clsiServerId,omitempty"`
	CompileGroup      clsiTypes.CompileGroup `json:"compileGroup,omitempty"`
	PDFDownloadDomain PDFDownloadDomain      `json:"pdfDownloadDomain,omitempty"`
}
