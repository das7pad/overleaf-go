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

package user

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AlphaProgramField struct {
	AlphaProgram bool `json:"alphaProgram" bson:"alphaProgram"`
}

type BetaProgramField struct {
	BetaProgram bool `json:"betaProgram" bson:"betaProgram"`
}

type EditorConfigField struct {
	EditorConfig EditorConfig `json:"ace" bson:"ace"`
}

type EmailField struct {
	Email string `json:"email" bson:"email"`
}

type EpochField struct {
	Epoch int64 `bson:"epoch"`
}

type FeaturesField struct {
	Features Features `json:"features" bson:"features"`
}

type FirstNameField struct {
	FirstName string `json:"first_name" bson:"first_name"`
}

type IdField struct {
	Id primitive.ObjectID `json:"_id" bson:"_id"`
}

type IsAdminField struct {
	IsAdmin bool `json:"isAdmin" bson:"isAdmin"`
}

type LastNameField struct {
	LastName string `json:"last_name" bson:"last_name"`
}
