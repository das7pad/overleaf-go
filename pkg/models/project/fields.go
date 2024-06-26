// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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

	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	spellingTypes "github.com/das7pad/overleaf-go/services/spelling/pkg/types"
)

type Member struct {
	AccessSource   AccessSource
	PrivilegeLevel sharedTypes.PrivilegeLevel
	Archived       bool
	Trashed        bool
}

type CompilerField struct {
	Compiler sharedTypes.Compiler `json:"compiler"`
}

type ContentLockedAtField struct {
	ContentLockedAt *time.Time `json:"contentLockedAt"`
}

type CreatedAtField struct {
	CreatedAt time.Time
}

type DeletedAtField struct {
	DeletedAt time.Time
}

type DeletedDocsField struct {
	DeletedDocs []CommonTreeFields `json:"deletedDocs"`
}

type EditableField struct {
	Editable bool `json:"editable"`
}

type EpochField struct {
	Epoch int64
}

type IdField struct {
	Id sharedTypes.UUID `json:"_id"`
}

type ImageNameField struct {
	ImageName sharedTypes.ImageName `json:"imageName"`
}

type LastOpenedField struct {
	LastOpened time.Time
}

type LastUpdatedAtField struct {
	LastUpdatedAt time.Time
}

type LastUpdatedByField struct {
	LastUpdatedBy sharedTypes.UUID
}

type NameField struct {
	Name Name `json:"name"`
}

type OwnerIdField struct {
	OwnerId sharedTypes.UUID `json:"owner_ref"`
}

type OwnerField struct {
	Owner user.WithPublicInfo `json:"owner"`
}

type OwnerFeaturesField struct {
	OwnerFeatures user.Features `json:"features"`
}

type PublicAccessLevelField struct {
	PublicAccessLevel PublicAccessLevel `json:"publicAccessLevel"`
}

type RootDocIdField struct {
	RootDocId sharedTypes.UUID `json:"rootDoc_id"`
}

type RootDocField struct {
	RootDoc RootDoc `json:"root_doc"`
}

type SpellCheckLanguageField struct {
	SpellCheckLanguage spellingTypes.SpellCheckLanguage `json:"spellCheckLanguage"`
}

type TokensField struct {
	Tokens Tokens `json:"tokens"`
}

type RootFolderField struct {
	RootFolder Folder
}

type FoldersField struct {
	Folders []Folder `json:"-"`
}

type DocsField struct {
	Docs []Doc
}

type FilesField struct {
	Files []FileRef
}

type VersionField struct {
	Version sharedTypes.Version `json:"version"`
}
