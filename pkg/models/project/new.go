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

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

const (
	DefaultCompiler = sharedTypes.PDFLatex
)

func NewProject(ownerId edgedb.UUID) *ForCreation {
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
		LastUpdatedAtField: LastUpdatedAtField{
			LastUpdatedAt: time.Now().UTC(),
		},
		OwnerField: OwnerField{
			Owner: user.WithPublicInfoAndFeatures{
				WithPublicInfo: user.WithPublicInfo{
					IdField: user.IdField{Id: ownerId},
				},
			},
		},
		PublicAccessLevelField: PublicAccessLevelField{
			PublicAccessLevel: PrivateAccess,
		},
		SpellCheckLanguageField: SpellCheckLanguageField{
			SpellCheckLanguage: "inherit",
		},
		RootFolderField: RootFolderField{
			RootFolder: RootFolder{
				Folder: *NewFolder(""),
			},
		},
		VersionField: VersionField{
			Version: 1,
		},
	}
}
