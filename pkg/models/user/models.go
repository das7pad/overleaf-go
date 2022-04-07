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
	"context"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type ProjectListViewCaller struct {
	WithPublicInfo `edgedb:"$inline"`
	EmailsField    `edgedb:"$inline"`
}

type WithPublicInfo struct {
	EmailField     `edgedb:"email"`
	FirstNameField `edgedb:"$inline"`
	IdField        `edgedb:"$inline"`
	LastNameField  `edgedb:"$inline"`
}

type WithPublicInfoAndFeatures struct {
	FeaturesField  `edgedb:"$inline"`
	WithPublicInfo `edgedb:"$inline"`
}

type WithLoadEditorInfo struct {
	edgedb.Optional
	AlphaProgramField         `edgedb:"$inline"`
	BetaProgramField          `edgedb:"$inline"`
	EditorConfigField         `edgedb:"$inline"`
	EpochField                `edgedb:"$inline"`
	IsAdminField              `edgedb:"$inline"`
	WithPublicInfoAndFeatures `edgedb:"$inline"`
}

type WithLoginInfo struct {
	ForSession          `edgedb:"$inline"`
	HashedPasswordField `edgedb:"$inline"`
	MustReconfirmField  `edgedb:"$inline"`
}

func (l *WithLoginInfo) selectByEmail(ctx context.Context, c *edgedb.Client, email sharedTypes.Email) error {
	return c.QuerySingle(ctx, `
select User {

}
filter .email.email = <str>$0
`,
		l,
		email)
}

type ForSession struct {
	EpochField     `edgedb:"$inline"`
	IsAdminField   `edgedb:"$inline"`
	WithPublicInfo `edgedb:"$inline"`
}

type WithEpochAndFeatures struct {
	EpochField    `edgedb:"$inline"`
	FeaturesField `edgedb:"$inline"`
}

type ForCreation struct {
	ForSession          `edgedb:"$inline"`
	AuditLogField       `edgedb:"$inline"`
	EditorConfigField   `edgedb:"$inline"`
	EmailsField         `edgedb:"$inline"`
	FeaturesField       `edgedb:"$inline"`
	HashedPasswordField `edgedb:"$inline"`
	LastLoggedInField   `edgedb:"$inline"`
	LastLoginIpField    `edgedb:"$inline"`
	LoginCountField     `edgedb:"$inline"`
	SignUpDateField     `edgedb:"$inline"`
}

type ForDeletion struct {
	AlphaProgramField   `edgedb:"$inline"`
	AuditLogField       `edgedb:"$inline"`
	BetaProgramField    `edgedb:"$inline"`
	EditorConfigField   `edgedb:"$inline"`
	EmailField          `edgedb:"email"`
	EmailsField         `edgedb:"$inline"`
	EpochField          `edgedb:"$inline"`
	FeaturesField       `edgedb:"$inline"`
	FirstNameField      `edgedb:"$inline"`
	HashedPasswordField `edgedb:"$inline"`
	IdField             `edgedb:"$inline"`
	IsAdminField        `edgedb:"$inline"`
	LastLoggedInField   `edgedb:"$inline"`
	LastLoginIpField    `edgedb:"$inline"`
	LastNameField       `edgedb:"$inline"`
	LoginCountField     `edgedb:"$inline"`
	MustReconfirmField  `edgedb:"$inline"`
	SignUpDateField     `edgedb:"$inline"`
}

type ForEmailChange struct {
	EmailField `edgedb:"email"`
	EpochField `edgedb:"$inline"`
	IdField    `edgedb:"$inline"`
}

type ForPasswordChange struct {
	WithPublicInfo      `edgedb:"$inline"`
	EpochField          `edgedb:"$inline"`
	HashedPasswordField `edgedb:"$inline"`
}

type WithNames struct {
	FirstNameField `edgedb:"$inline"`
	LastNameField  `edgedb:"$inline"`
}

type ForSettingsPage struct {
	WithPublicInfo   `edgedb:"$inline"`
	BetaProgramField `edgedb:"$inline"`
}

type ForActivateUserPage struct {
	EmailField      `edgedb:"email"`
	LoginCountField `edgedb:"$inline"`
}
