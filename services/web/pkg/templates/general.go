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
	"io"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type General400Data struct {
	NoJsLayoutData
	Message string
}

func (d *General400Data) Render() (string, error) {
	return renderOff("400.gohtml", 10*1024, d)
}

type General404Data struct {
	MarketingLayoutData
}

func (d *General404Data) Render() (string, error) {
	return renderOff("general/404.gohtml", 10*1024, d)
}

type General500Data struct {
	NoJsLayoutData
}

func (d *General500Data) Render() (string, error) {
	return renderOff("general/500.gohtml", 10*1024, d)
}

type GeneralUnsupportedBrowserData struct {
	NoJsLayoutData
	FromURL *sharedTypes.URL
}

func (d *GeneralUnsupportedBrowserData) Render(w io.Writer) error {
	return render("general/unsupported-browser.gohtml", w, d)
}
