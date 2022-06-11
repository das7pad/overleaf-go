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
	"database/sql"
	"time"

	"github.com/das7pad/overleaf-go/pkg/models/projectInvite"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	spellingTypes "github.com/das7pad/overleaf-go/services/spelling/pkg/types"
)

type Member struct {
	Archived       bool
	AccessSource   AccessSource
	PrivilegeLevel sharedTypes.PrivilegeLevel
	CanWrite       bool
	IsTokenMember  IsTokenMember
	Trashed        bool
}

type AccessReadAndWriteField struct {
	AccessReadAndWrite Refs `edgedb:"access_rw"`
}

type AccessReadOnlyField struct {
	AccessReadOnly Refs `edgedb:"access_ro"`
}

type CompilerField struct {
	Compiler sharedTypes.Compiler `json:"compiler" edgedb:"compiler"`
}

type DeletedAtField struct {
	DeletedAt sql.NullTime `edgedb:"deleted_at"`
}

type DeletedDocsField struct {
	DeletedDocs []CommonTreeFields `json:"deletedDocs" edgedb:"deleted_docs"`
}

type EpochField struct {
	Epoch int64 `edgedb:"epoch"`
}

type IdField struct {
	Id sharedTypes.UUID `json:"_id" edgedb:"id"`
}

type ImageNameField struct {
	ImageName sharedTypes.ImageName `json:"imageName" edgedb:"image_name"`
}

type InvitesField struct {
	Invites []projectInvite.WithoutToken `json:"invites" edgedb:"invites"`
}

type LastOpenedField struct {
	LastOpened time.Time `edgedb:"last_opened"`
}

type LastUpdatedAtField struct {
	LastUpdatedAt *time.Time `edgedb:"last_updated_at"`
}

type LastUpdatedByField struct {
	LastUpdatedBy sharedTypes.UUID `edgedb:"last_updated_by"`
}

type NameField struct {
	Name Name `json:"name" edgedb:"name"`
}

type OwnerIdField struct {
	OwnerId sharedTypes.UUID `json:"owner_ref"`
}

type OwnerFeaturesField struct {
	OwnerFeatures user.Features `json:"features"`
}

//goland:noinspection SpellCheckingInspection
type PublicAccessLevelField struct {
	PublicAccessLevel PublicAccessLevel `json:"publicAccesLevel" edgedb:"public_access_level"`
}

type RootDocIdField struct {
	// Virtual field
	RootDocId sharedTypes.UUID `json:"rootDoc_id"`
}

type RootDocField struct {
	RootDoc RootDoc `json:"root_doc" edgedb:"root_doc"`
}

type SpellCheckLanguageField struct {
	SpellCheckLanguage spellingTypes.SpellCheckLanguage `json:"spellCheckLanguage" edgedb:"spell_check_language"`
}

type TokensField struct {
	Tokens Tokens `json:"tokens" edgedb:"tokens"`
}

type RootFolderField struct {
	RootFolder Folder `edgedb:"root_folder"`
}

type FoldersField struct {
	Folders []Folder `json:"-" edgedb:"folders"`
}

type DocsField struct {
	Docs []Doc `edgedb:"docs"`
}

type FilesField struct {
	Files []FileRef `edgedb:"files"`
}

type VersionField struct {
	Version sharedTypes.Version `json:"version" edgedb:"version"`
}
