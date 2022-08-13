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

package project

import (
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

const (
	DefaultCompiler = sharedTypes.PDFLaTeX
)

func NewProject(ownerId sharedTypes.UUID) (*ForCreation, error) {
	id, err := sharedTypes.GenerateUUID()
	if err != nil {
		return nil, err
	}
	return &ForCreation{
		CompilerField: CompilerField{
			Compiler: DefaultCompiler,
		},
		IdField: IdField{
			Id: id,
		},
		OwnerIdField: OwnerIdField{
			OwnerId: ownerId,
		},
		SpellCheckLanguageField: SpellCheckLanguageField{
			SpellCheckLanguage: "inherit",
		},
		RootFolderField: RootFolderField{
			RootFolder: NewFolder(""),
		},
	}, nil
}
