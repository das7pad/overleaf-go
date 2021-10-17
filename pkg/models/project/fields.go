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

type ActiveField struct {
	Active bool `bson:"active"`
}

type ArchivedByField struct {
	ArchivedBy Refs `bson:"archived"`
}

//goland:noinspection SpellCheckingInspection
type CollaboratorRefsField struct {
	CollaboratorRefs Refs `bson:"collaberator_refs"`
}

type CompilerField struct {
	// TODO: move Compiler into sharedTypes
	Compiler clsiTypes.Compiler `json:"compiler" bson:"compiler"`
}

type EpochField struct {
	Epoch int64 `bson:"epoch"`
}

type IdField struct {
	Id primitive.ObjectID `json:"_id" bson:"_id"`
}

type ImageNameField struct {
	// TODO: move ImageName into sharedTypes
	ImageName clsiTypes.ImageName `json:"imageName" bson:"imageName"`
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
	Name string `json:"name" bson:"name"`
}

type OwnerRefField struct {
	OwnerRef primitive.ObjectID `bson:"owner_ref" json:"owner_ref"`
}

//goland:noinspection SpellCheckingInspection
type PublicAccessLevelField struct {
	PublicAccessLevel PublicAccessLevel `json:"publicAccesLevel" bson:"publicAccesLevel"`
}

type ReadOnlyRefsField struct {
	ReadOnlyRefs Refs `bson:"readOnly_refs"`
}

type RootDocIdField struct {
	RootDocId primitive.ObjectID `json:"rootDoc_id" bson:"rootDoc_id"`
}

type SpellCheckLanguageField struct {
	SpellCheckLanguage string `json:"spellCheckLanguage" bson:"spellCheckLanguage"`
}

type TokenAccessReadAndWriteRefsField struct {
	TokenAccessReadAndWriteRefs Refs `bson:"tokenAccessReadAndWrite_refs"`
}

type TokenAccessReadOnlyRefsField struct {
	TokenAccessReadOnlyRefs Refs `bson:"tokenAccessReadOnly_refs"`
}

type TokensField struct {
	Tokens Tokens `json:"tokens" bson:"tokens"`
}

type TrackChangesStateField struct {
	TrackChangesState map[string]bool `json:"trackChangesState" bson:"track_changes"`
}

type TrashedByField struct {
	TrashedBy Refs `bson:"trashed"`
}

type TreeField struct {
	RootFolder []Folder `json:"rootFolder" bson:"rootFolder"`
}
