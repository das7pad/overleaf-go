// Golang port of Overleaf
// Copyright (C) 2021-2023 Jakob Ackermann <das7pad@outlook.com>
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

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	spellingTypes "github.com/das7pad/overleaf-go/services/spelling/pkg/types"
)

type ActiveField struct {
	Active bool `bson:"active"`
}

type ArchivedByField struct {
	ArchivedBy Refs `bson:"archived"`
}

type AuditLogField struct {
	AuditLog []AuditLogEntry `bson:"auditLog"`
}

//goland:noinspection SpellCheckingInspection
type CollaboratorRefsField struct {
	CollaboratorRefs Refs `bson:"collaberator_refs"`
}

type CompilerField struct {
	Compiler sharedTypes.Compiler `bson:"compiler"`
}

type EpochField struct {
	Epoch int64 `bson:"epoch"`
}

type IdField struct {
	Id primitive.ObjectID `bson:"_id"`
}

type ImageNameField struct {
	ImageName sharedTypes.ImageName `bson:"imageName"`
}

type LastOpenedField struct {
	LastOpened time.Time `bson:"lastOpened"`
}

type LastUpdatedAtField struct {
	LastUpdatedAt time.Time `bson:"lastUpdated"`
}

type LastUpdatedByField struct {
	LastUpdatedBy primitive.ObjectID `bson:"lastUpdatedBy"`
}

type NameField struct {
	Name Name `bson:"name"`
}

type OwnerRefField struct {
	OwnerRef primitive.ObjectID `bson:"owner_ref"`
}

//goland:noinspection SpellCheckingInspection
type PublicAccessLevelField struct {
	PublicAccessLevel PublicAccessLevel `bson:"publicAccesLevel"`
}

type ReadOnlyRefsField struct {
	ReadOnlyRefs Refs `bson:"readOnly_refs"`
}

type RootDocIdField struct {
	RootDocId primitive.ObjectID `bson:"rootDoc_id"`
}

type SpellCheckLanguageField struct {
	SpellCheckLanguage spellingTypes.SpellCheckLanguage `bson:"spellCheckLanguage"`
}

type TokenAccessReadAndWriteRefsField struct {
	TokenAccessReadAndWriteRefs Refs `bson:"tokenAccessReadAndWrite_refs"`
}

type TokenAccessReadOnlyRefsField struct {
	TokenAccessReadOnlyRefs Refs `bson:"tokenAccessReadOnly_refs"`
}

type TokensField struct {
	Tokens Tokens `bson:"tokens"`
}

type TrashedByField struct {
	TrashedBy Refs `bson:"trashed"`
}

type TreeField struct {
	RootFolder []*Folder `bson:"rootFolder"`
}

type VersionField struct {
	Version sharedTypes.Version `bson:"version"`
}
