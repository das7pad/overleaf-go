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
	"github.com/das7pad/overleaf-go/pkg/csp"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/pkg/translations"
)

type NavOptions struct {
	HeaderExtras []NavElementWithDropDown `json:"header_extras"`
	LeftFooter   []NavElement             `json:"left_footer"`
	RightFooter  []NavElement             `json:"right_footer"`
	Title        string                   `json:"title"`
}

type NavElement struct {
	Class string `json:"class"`
	Label string `json:"label"`
	Text  string `json:"text"`
	URL   string `json:"url"`
}

type NavElementWithDivider struct {
	NavElement
	Divider bool `json:"divider"`
}

type NavElementWithDropDown struct {
	NavElement
	Dropdown []NavElementWithDivider `json:"dropdown"`
}

type I18nSubDomainLang struct {
	Hide    bool   `json:"hide"`
	LngCode string `json:"lng_code"`
}

type I18nOptions struct {
	DefaultLang   string              `json:"default_lang"`
	SubdomainLang []I18nSubDomainLang `json:"subdomain_lang"`
}

func (o *I18nOptions) Validate() error {
	if err := (translations.Languages{o.DefaultLang}).Validate(); err != nil {
		return errors.Tag(err, "invalid default_lang")
	}
	if err := o.Languages().Validate(); err != nil {
		return errors.Tag(err, "subdomain_lang contains invalid entry")
	}
	return nil
}

func (o *I18nOptions) Languages() translations.Languages {
	allowed := make(map[string]bool, len(o.SubdomainLang))
	allowed[o.DefaultLang] = true
	for _, lang := range o.SubdomainLang {
		allowed[lang.LngCode] = true
	}
	flat := make(translations.Languages, 0, len(allowed))
	for s := range allowed {
		flat = append(flat, s)
	}
	return flat
}

type SentryFrontendOptions struct {
	Dsn                string `json:"dsn"`
	Environment        string `json:"environment,omitempty"`
	Release            string `json:"release,omitempty"`
	Commit             string `json:"commit,omitempty"`
	AllowedOriginRegex string `json:"allowedOriginRegex,omitempty"`
}

type PublicSentryOptions struct {
	Frontend SentryFrontendOptions `json:"frontend"`
}

type PublicSettings struct {
	AppName                   string
	AdminEmail                sharedTypes.Email
	CDNURL                    sharedTypes.URL
	CSPs                      csp.CSPs
	EditorSettings            EditorSettings
	I18n                      I18nOptions
	Nav                       NavOptions
	Sentry                    PublicSentryOptions
	SiteURL                   sharedTypes.URL
	StatusPageURL             sharedTypes.URL
	TranslatedLanguages       map[string]string
	ZipFileSizeLimit          int64
	EmailConfirmationDisabled bool
	RegistrationDisabled      bool
	RobotsNoindex             bool
}

func (s *PublicSettings) ShowLanguagePicker() bool {
	return len(s.I18n.SubdomainLang) > 1
}
