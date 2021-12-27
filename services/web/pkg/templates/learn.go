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
	"html/template"
)

type LearnPageData struct {
	MarketingLayoutData

	PageContent     template.HTML
	ContentsContent template.HTML
}

func (d *LearnPageData) Render() (string, error) {
	n := 11*1024 +
		len(d.PageContent) +
		len(d.ContentsContent) +
		4*len(d.Title) +
		4*len(d.TitleLocale)
	return render("learn/page.gohtml", n, d)
}
