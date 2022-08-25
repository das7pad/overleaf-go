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

package templates

import (
	"github.com/das7pad/overleaf-go/pkg/models/systemMessage"
)

type AdminManageSiteData struct {
	MarketingLayoutData

	SystemMessages []systemMessage.Full
}

func (d *AdminManageSiteData) Render() ([]byte, string, error) {
	return render("admin/manageSite.gohtml", 7*1024, d)
}

type AdminRegisterUsersData struct {
	AngularLayoutData
}

func (d *AdminRegisterUsersData) Render() ([]byte, string, error) {
	return render("admin/registerUsers.gohtml", 7*1024, d)
}
