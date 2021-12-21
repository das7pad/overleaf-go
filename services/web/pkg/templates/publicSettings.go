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
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type NavOptions struct {
	HeaderExtras []NavElementWithDropDown
	LeftFooter   []NavElement
	RightFooter  []NavElement
	Title        string
}

type NavElement struct {
	Class string
	Label string
	Text  string
	URL   string
}

type NavElementWithDivider struct {
	NavElement
	Divider bool
}

type NavElementWithDropDown struct {
	NavElement
	Dropdown []NavElementWithDivider
}

type I18nSubDomainLang struct {
	Hide    bool
	LngCode string
}

type I18nOptions struct {
	SubdomainLang []I18nSubDomainLang
}

type SentryFrontendOptions struct {
	Dsn                string `json:"dsn"`
	Environment        string `json:"environment,omitempty"`
	Release            string `json:"release,omitempty"`
	Commit             string `json:"commit,omitempty"`
	AllowedOriginRegex string `json:"allowedOriginRegex,omitempty"`
}

type PublicSentryOptions struct {
	Frontend SentryFrontendOptions
}

type PublicSettings struct {
	AppName             string
	AdminEmail          sharedTypes.Email
	CDNURL              sharedTypes.URL
	I18n                I18nOptions
	Nav                 NavOptions
	RobotsNoindex       bool
	Sentry              PublicSentryOptions
	StatusPageURL       sharedTypes.URL
	TranslatedLanguages map[string]string
}

func (s *PublicSettings) ShowLanguagePicker() bool {
	return len(s.I18n.SubdomainLang) > 1
}
