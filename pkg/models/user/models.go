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

type withEmailFields struct {
	EmailField  `bson:"inline"`
	EmailsField `bson:"inline"`
}

type withLastLoginInfo struct {
	LastLoggedInField `bson:"inline"`
	LastLoginIpField  `bson:"inline"`
}

type withIdAndEpoch struct {
	IdField    `bson:"inline"`
	EpochField `bson:"inline"`
}

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
	ForSession          `bson:"inline"`
	HashedPasswordField `bson:"inline"`
	MustReconfirmField  `bson:"inline"`
}

type ForSession struct {
	EpochField      `bson:"inline"`
	IsAdminField    `bson:"inline"`
	ReferralIdField `bson:"inline"`
	WithPublicInfo  `bson:"inline"`
}

type WithEpochAndFeatures struct {
	EpochField    `bson:"inline"`
	FeaturesField `bson:"inline"`
}

type ForCreation struct {
	ForSession          `bson:"inline"`
	AuditLogField       `bson:"inline"`
	EditorConfigField   `bson:"inline"`
	EmailsField         `bson:"inline"`
	FeaturesField       `bson:"inline"`
	HashedPasswordField `bson:"inline"`
	LastLoggedInField   `bson:"inline"`
	LastLoginIpField    `bson:"inline"`
	LoginCountField     `bson:"inline"`
	SignUpDateField     `bson:"inline"`
}

type ForDeletion struct {
	AlphaProgramField   `bson:"inline"`
	AuditLogField       `bson:"inline"`
	BetaProgramField    `bson:"inline"`
	EditorConfigField   `bson:"inline"`
	EmailField          `bson:"inline"`
	EmailsField         `bson:"inline"`
	EpochField          `bson:"inline"`
	FeaturesField       `bson:"inline"`
	FirstNameField      `bson:"inline"`
	HashedPasswordField `bson:"inline"`
	IdField             `bson:"inline"`
	IsAdminField        `bson:"inline"`
	LastLoggedInField   `bson:"inline"`
	LastLoginIpField    `bson:"inline"`
	LastNameField       `bson:"inline"`
	LoginCountField     `bson:"inline"`
	MustReconfirmField  `bson:"inline"`
	ReferralIdField     `bson:"inline"`
	SignUpDateField     `bson:"inline"`
}

type ForEmailChange struct {
	EmailField `bson:"inline"`
	EpochField `bson:"inline"`
	IdField    `bson:"inline"`
}

type ForPasswordChange struct {
	WithPublicInfo      `bson:"inline"`
	EpochField          `bson:"inline"`
	HashedPasswordField `bson:"inline"`
}

type WithNames struct {
	FirstNameField `bson:"inline"`
	LastNameField  `bson:"inline"`
}

type ForSettingsPage struct {
	WithPublicInfo   `bson:"inline"`
	BetaProgramField `bson:"inline"`
}

type ForActivateUserPage struct {
	EmailField      `bson:"inline"`
	LoginCountField `bson:"inline"`
}
