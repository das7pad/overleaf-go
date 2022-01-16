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
	"encoding/json"
	"net/url"
)

type OpenInOverleafDocumentationData struct {
	AngularLayoutData

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

type OpenInOverleafGatewayData struct {
	AngularLayoutData

	Query *url.Values
	Body  *json.RawMessage
}

func (d *OpenInOverleafGatewayData) Meta() []metaEntry {
	m := d.AngularLayoutData.Meta()
	if d.Query != nil {
		m = append(m, metaEntry{
			Name:    "ol-oio-params",
			Type:    jsonContentType,
			Content: d.Query,
		})
	} else {
		m = append(m, metaEntry{
			Name:    "ol-oio-params",
			Type:    jsonContentType,
			Content: d.Body,
		})
	}
	return m
}

func (d *OpenInOverleafGatewayData) Render() ([]byte, error) {
	d.HideFooter = true
	d.HideNavBar = true
	return render("openInOverleaf/gateway.gohtml", 100*1024, d)
}
