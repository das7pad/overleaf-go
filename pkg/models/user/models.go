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

type ProjectListViewCaller struct {
	WithPublicInfo `bson:"inline"`
	EmailsField    `bson:"inline"`
}

type WithPublicInfo struct {
	EmailField     `bson:"inline"`
	FirstNameField `bson:"inline"`
	IdField        `bson:"inline"`
	LastNameField  `bson:"inline"`
}

type WithPublicInfoAndFeatures struct {
	FeaturesField  `bson:"inline"`
	WithPublicInfo `bson:"inline"`
}

type WithLoadEditorInfo struct {
	AlphaProgramField         `bson:"inline"`
	BetaProgramField          `bson:"inline"`
	EditorConfigField         `bson:"inline"`
	EpochField                `bson:"inline"`
	IsAdminField              `bson:"inline"`
	WithPublicInfoAndFeatures `bson:"inline"`
}

type WithLoginInfo struct {
	EpochField          `bson:"inline"`
	IsAdminField        `bson:"inline"`
	MustReconfirmField  `bson:"inline"`
	ReferralIdField     `bson:"inline"`
	HashedPasswordField `bson:"inline"`
	WithPublicInfo      `bson:"inline"`
}

type WithEpochAndFeatures struct {
	EpochField    `bson:"inline"`
	FeaturesField `bson:"inline"`
}
