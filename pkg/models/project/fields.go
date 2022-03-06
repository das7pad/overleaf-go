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

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	spellingTypes "github.com/das7pad/overleaf-go/services/spelling/pkg/types"
)

type ActiveField struct {
	Active bool `edgedb:"active"`
}

type ArchivedByField struct {
	ArchivedBy Refs `edgedb:"archived"`
}

type AuditLogField struct {
	AuditLog []AuditLogEntry `edgedb:"auditLog"`
}

//goland:noinspection SpellCheckingInspection
type CollaboratorRefsField struct {
	CollaboratorRefs Refs `edgedb:"collaberator_refs"`
}

type CompilerField struct {
	Compiler sharedTypes.Compiler `json:"compiler" edgedb:"compiler"`
}

type EpochField struct {
	Epoch int64 `edgedb:"epoch"`
}

type IdField struct {
	Id edgedb.UUID `json:"_id" edgedb:"id"`
}

type ImageNameField struct {
	ImageName sharedTypes.ImageName `json:"imageName" edgedb:"imageName"`
}

type LastOpenedField struct {
	LastOpened time.Time `edgedb:"lastOpened"`
}

type LastUpdatedAtField struct {
	LastUpdatedAt time.Time `edgedb:"lastUpdated"`
}

type LastUpdatedByField struct {
	LastUpdatedBy edgedb.UUID `edgedb:"lastUpdatedBy"`
}

type NameField struct {
	Name Name `json:"name" edgedb:"name"`
}

type OwnerRefField struct {
	OwnerRef edgedb.UUID `edgedb:"owner_ref" json:"owner_ref"`
}

//goland:noinspection SpellCheckingInspection
type PublicAccessLevelField struct {
	PublicAccessLevel PublicAccessLevel `json:"publicAccesLevel" edgedb:"publicAccesLevel"`
}

type ReadOnlyRefsField struct {
	ReadOnlyRefs Refs `edgedb:"readOnly_refs"`
}

type RootDocIdField struct {
	RootDocId edgedb.UUID `json:"rootDoc_id" edgedb:"rootDoc_id"`
}

type RootDocField struct {
	RootDoc *Doc `json:"root_doc" edgedb:"root_doc"`
}

type SpellCheckLanguageField struct {
	SpellCheckLanguage spellingTypes.SpellCheckLanguage `json:"spellCheckLanguage" edgedb:"spellCheckLanguage"`
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
	TrackChangesState TrackChangesState `json:"trackChangesState" edgedb:"track_changes"`
}

type TrashedByField struct {
	TrashedBy Refs `edgedb:"trashed"`
}

type RootFolderField struct {
	RootFolder *Folder `edgedb:"root_folder"`
}

type FoldersField struct {
	Folders []*Folder `edgedb:"folders"`
}

type DocsField struct {
	Docs []*Doc `edgedb:"docs"`
}

type FilesField struct {
	Files []*FileRef `edgedb:"files"`
}

type AnyFoldersField struct {
	AnyFolders []*Folder `edgedb:"any_folders"`
}

type TreeField struct {
	RootFolder []*Folder `json:"rootFolder" edgedb:"rootFolder"`
}

type VersionField struct {
	Version sharedTypes.Version `json:"version" edgedb:"version"`
}
