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

package templates

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type CommonData struct {
	Settings *types.PublicSettings

	CurrentLngCode        string
	RobotsNoindexNofollow bool
	Title                 string
	Viewport              bool
}

func (d *CommonData) GetCurrentLngCode() string {
	return d.CurrentLngCode
}

type JsLayoutData struct {
	CommonData
	UsersEmail       sharedTypes.Email
	UserId           primitive.ObjectID
	IsAdmin          bool
	CustomEntrypoint string
}

func (d *JsLayoutData) LoggedIn() bool {
	return !d.UserId.IsZero()
}

func (d *JsLayoutData) UserIdNoZero() string {
	if d.UserId.IsZero() {
		return ""
	}
	return d.UserId.Hex()
}

type MarketingLayoutData struct {
	JsLayoutData
}

func (d *MarketingLayoutData) Entrypoint() string {
	if d.CustomEntrypoint != "" {
		return d.CustomEntrypoint
	}
	return "frontend/js/marketing.js"
}

type AngularLayoutData struct {
	JsLayoutData
}

func (d *AngularLayoutData) Entrypoint() string {
	if d.CustomEntrypoint != "" {
		return d.CustomEntrypoint
	}
	return "frontend/js/main.js"
}

type NoJsLayoutData struct {
	CommonData
}
