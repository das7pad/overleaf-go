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

package user

import (
	"github.com/das7pad/overleaf-go/pkg/models/oneTimeToken"
)

type ProjectListViewCaller struct {
	WithPublicInfo
	EmailConfirmedAtField
}

type WithPublicInfo struct {
	EmailField
	FirstNameField
	IdField
	LastNameField
}

type WithPublicInfoAndFeatures struct {
	FeaturesField
	WithPublicInfo
}

type WithLoadEditorInfo struct {
	EditorConfigField
	AlphaProgramField
	BetaProgramField
	IsAdminField
	EpochField
	WithPublicInfoAndFeatures
}

type WithLoginInfo struct {
	ForSession
	HashedPasswordField
	MustReconfirmField
}

type ForSession struct {
	EpochField
	IsAdminField
	WithPublicInfo
}

type ForCreation struct {
	ForSession
	AuditLogField
	EditorConfigField
	EmailsField
	FeaturesField
	HashedPasswordField
	LastLoggedInField
	LastLoginIpField
	LoginCountField
	SignUpDateField

	oneTimeToken.OneTimeToken
	OneTimeTokenUse string
}

type ForEmailChange struct {
	EmailField
	EpochField
	IdField
}

type ForPasswordChange struct {
	WithPublicInfo
	EpochField
	HashedPasswordField
}

type WithNames struct {
	FirstNameField
	LastNameField
}

type ForSettingsPage struct {
	WithPublicInfo
	BetaProgramField
}

type ForActivateUserPage struct {
	EmailField
	LoginCountField
}
