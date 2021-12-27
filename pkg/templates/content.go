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

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/session"
)

const (
	stringContentType metaType = iota
	jsonContentType
)

type metaType int

type metaEntry struct {
	Name    string
	Type    metaType
	Content interface{}
}

func (m metaEntry) ContentAsString() (string, error) {
	switch m.Type {
	case stringContentType:
		return m.Content.(string), nil
	case jsonContentType:
		if b, ok := m.Content.(*json.RawMessage); ok {
			return string(*b), nil
		}
		b, err := json.Marshal(m.Content)
		if err != nil {
			return "", errors.Tag(err, "cannot marshal .Content for "+m.Name)
		}
		return string(b), nil
	default:
		return "", errors.New("unexpected .Type for " + m.Name)
	}
}

func (m metaEntry) TypeAsString() (string, error) {
	switch m.Type {
	case stringContentType:
		return "string", nil
	case jsonContentType:
		return "json", nil
	default:
		return "", errors.New("unexpected .Type for " + m.Name)
	}
}

type CommonData struct {
	Settings *PublicSettings

	RobotsNoindexNofollow bool
	SessionUser           *session.User
	ThemeModifier         string
	Title                 string
	TitleLocale           string
	Viewport              bool
}

func (d *CommonData) CurrentLngCode() string {
	if u := d.SessionUser; u != nil && u.Language != "" {
		return u.Language
	}
	return "en"
}

func (d *CommonData) IsAdmin() bool {
	if d.SessionUser == nil {
		return false
	}
	return d.SessionUser.IsAdmin
}

func (d *CommonData) LoggedIn() bool {
	if d.SessionUser == nil {
		return false
	}
	return !d.SessionUser.Id.IsZero()
}

func (d *CommonData) Meta() []metaEntry {
	var out []metaEntry
	if d.Settings.Sentry.Frontend.Dsn != "" {
		out = append(out, metaEntry{
			Name:    "ol-sentry",
			Type:    jsonContentType,
			Content: d.Settings.Sentry.Frontend,
		})
	}
	return out
}

type AngularLayoutData struct {
	CommonData
}

func (d *AngularLayoutData) Entrypoint() string {
	return "frontend/js/main.js"
}

func (d *AngularLayoutData) CSP() string {
	return d.Settings.CSPs.Angular
}

type MarketingLayoutData struct {
	CommonData
}

func (d *MarketingLayoutData) Entrypoint() string {
	return "frontend/js/marketing.js"
}

func (d *MarketingLayoutData) CSP() string {
	return d.Settings.CSPs.Marketing
}

type NoJsLayoutData struct {
	CommonData
}

func (d *NoJsLayoutData) CSP() string {
	return d.Settings.CSPs.NoJs
}
