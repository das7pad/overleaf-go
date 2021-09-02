// Golang port of the Overleaf web service
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
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type Timeout time.Duration

type SignedCompileProjectRequestOptions struct {
	CompileGroup clsiTypes.CompileGroup
	ProjectId    primitive.ObjectID
	UserId       primitive.ObjectID
	Timeout      clsiTypes.Timeout
}

type CompileProjectRequest struct {
	SignedCompileProjectRequestOptions

	AutoCompile bool
	Draft       clsiTypes.DraftModeFlag
	Compiler    clsiTypes.Compiler
	CheckMode   clsiTypes.CheckMode
	ImageName   clsiTypes.ImageName
	RootDocId   primitive.ObjectID
}

type ClsiServerId string
type PDFDownloadDomain string

type CompileProjectResponse struct {
	clsiTypes.CompileResponse
	ClsiServerId      ClsiServerId           `json:"clsiServerId"`
	CompileGroup      clsiTypes.CompileGroup `json:"compileGroup"`
	PDFDownloadDomain PDFDownloadDomain      `json:"pdfDownloadDomain"`
}
