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

	"github.com/das7pad/overleaf-go/pkg/models/projectInvite"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	spellingTypes "github.com/das7pad/overleaf-go/services/spelling/pkg/types"
)

type AccessReadAndWriteField struct {
	AccessReadAndWrite Refs `edgedb:"access_rw"`
}

type AccessReadOnlyField struct {
	AccessReadOnly Refs `edgedb:"access_ro"`
}

type AccessTokenReadAndWriteField struct {
	AccessTokenReadAndWrite Refs `edgedb:"access_token_rw"`
}

type AccessTokenReadOnlyField struct {
	AccessTokenReadOnly Refs `edgedb:"access_token_ro"`
}

type ActiveField struct {
	Active bool `edgedb:"active"`
}

type ArchivedByField struct {
	ArchivedBy Refs `edgedb:"archived"`
}

type ArchivedField struct {
	Archived bool `edgedb:"archived"`
}

type AuditLogField struct {
	AuditLog []AuditLogEntry `edgedb:"audit_log"`
}

//goland:noinspection SpellCheckingInspection
type CollaboratorRefsField struct {
	CollaboratorRefs Refs `edgedb:"collaberator_refs"`
}

type CompilerField struct {
	Compiler sharedTypes.Compiler `json:"compiler" edgedb:"compiler"`
}

type DeletedDocsField struct {
	DeletedDocs []CommonTreeFields `json:"deletedDocs" edgedb:"deleted_docs"`
}

type EpochField struct {
	Epoch int64 `edgedb:"epoch"`
}

type IdField struct {
	Id edgedb.UUID `json:"_id" edgedb:"id"`
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
	LastUpdatedAt time.Time `edgedb:"last_updated_at"`
}

type LastUpdatedByField struct {
	LastUpdatedBy user.WithPublicInfo `edgedb:"last_updated_by"`
}

type NameField struct {
	Name Name `json:"name" edgedb:"name"`
}

type OwnerField struct {
	Owner user.WithPublicInfoAndFeatures `edgedb:"owner" json:"owner"`
}

//goland:noinspection SpellCheckingInspection
type PublicAccessLevelField struct {
	PublicAccessLevel PublicAccessLevel `json:"publicAccesLevel" edgedb:"public_access_level"`
}

type ReadOnlyRefsField struct {
	ReadOnlyRefs Refs `edgedb:"readOnly_refs"`
}

type RootDocIdField struct {
	// Virtual field
	RootDocId edgedb.UUID `json:"rootDoc_id"`
}

type RootDocField struct {
	RootDoc RootDoc `json:"root_doc" edgedb:"root_doc"`
}

type SpellCheckLanguageField struct {
	SpellCheckLanguage spellingTypes.SpellCheckLanguage `json:"spellCheckLanguage" edgedb:"spell_check_language"`
}

type TokenAccessReadAndWriteRefsField struct {
	TokenAccessReadAndWriteRefs Refs `edgedb:"tokenAccessReadAndWrite_refs"`
}

type TokenAccessReadOnlyRefsField struct {
	TokenAccessReadOnlyRefs Refs `edgedb:"tokenAccessReadOnly_refs"`
}

type TokensField struct {
	Tokens Tokens `json:"tokens" edgedb:"tokens"`
}

type TrackChangesStateField struct {
	TrackChangesState TrackChangesState `json:"trackChangesState" edgedb:"track_changes_state"`
}

type TrashedByField struct {
	TrashedBy Refs `edgedb:"trashed_by"`
}

type TrashedField struct {
	Trashed bool `edgedb:"trashed"`
}

type RootFolderField struct {
	RootFolder RootFolder `edgedb:"root_folder"`
}

type FoldersField struct {
	Folders []Folder `json:"-" edgedb:"folders"`
}

type DocsField struct {
	Docs []*Doc `edgedb:"docs"`
}

type FilesField struct {
	Files []*FileRef `edgedb:"files"`
}

type TreeField struct {
	RootFolder []*Folder `json:"rootFolder" edgedb:"rootFolder"`
}

type VersionField struct {
	Version sharedTypes.Version `json:"version" edgedb:"version"`
}
