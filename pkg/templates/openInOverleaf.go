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
	"encoding/json"
)

type OpenInOverleafDocumentationData struct {
	MarketingLayoutData

	gwURL string
}

func (d *OpenInOverleafDocumentationData) GatewayURL() string {
	if d.gwURL == "" {
		d.gwURL = d.Settings.SiteURL.WithPath("/docs").String()
	}
	return d.gwURL
}

func (d *OpenInOverleafDocumentationData) Render() ([]byte, error) {
	return render("openInOverleaf/documentation.gohtml", 21*1024, d)
}

func (d *OpenInOverleafDocumentationData) Entrypoint() string {
	return "modules/open-in-overleaf/frontend/js/pages/documentation.js"
}

type OpenInOverleafGatewayData struct {
	MarketingLayoutData
	Params json.RawMessage
}

func (d *OpenInOverleafGatewayData) Meta() []metaEntry {
	m := d.MarketingLayoutData.Meta()
	m = append(m, metaEntry{
		Name:    "ol-oio-params",
		Type:    jsonContentType,
		Content: d.Params,
	})
	return m
}

func (d *OpenInOverleafGatewayData) Render() ([]byte, error) {
	d.HideFooter = true
	d.HideNavBar = true
	n := 10 * 1024
	if len(d.Params) > 6*1024 {
		n = 100 * 1024
	}
	return render("openInOverleaf/gateway.gohtml", n, d)
}

func (d *OpenInOverleafGatewayData) Entrypoint() string {
	return "modules/open-in-overleaf/frontend/js/pages/gateway.js"
}
