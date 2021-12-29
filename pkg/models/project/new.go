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

package project

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

const (
	DefaultCompiler = clsiTypes.PDFLatex
)

func NewProject(ownerId primitive.ObjectID) *ForCreation {
	return &ForCreation{
		ActiveField: ActiveField{
			Active: true,
		},
		CompilerField: CompilerField{
			Compiler: DefaultCompiler,
		},
		EpochField: EpochField{
			Epoch: 1,
		},
		IdField: IdField{
			Id: primitive.NewObjectID(),
		},
		LastUpdatedAtField: LastUpdatedAtField{
			LastUpdatedAt: time.Now().UTC(),
		},
		LastUpdatedByField: LastUpdatedByField{
			LastUpdatedBy: ownerId,
		},
		OwnerRefField: OwnerRefField{
			OwnerRef: ownerId,
		},
		PublicAccessLevelField: PublicAccessLevelField{
			PublicAccessLevel: PrivateAccess,
		},
		SpellCheckLanguageField: SpellCheckLanguageField{
			SpellCheckLanguage: "en",
		},
		TreeField: TreeField{
			RootFolder: []*Folder{
				NewFolder(""),
			},
		},
		VersionField: VersionField{
			Version: 1,
		},
	}
}
