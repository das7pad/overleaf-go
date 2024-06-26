// Golang port of Overleaf
// Copyright (C) 2022-2023 Jakob Ackermann <das7pad@outlook.com>
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

package siteLanguage

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/pkg/translations"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	SwitchLanguage(ctx context.Context, request *types.SwitchLanguageRequest, response *types.SwitchLanguageResponse) error
}

func New(options *types.Options) Manager {
	return &manager{
		allowed: options.I18n.Languages(),
		siteURL: options.SiteURL,
	}
}

type manager struct {
	allowed translations.Languages
	siteURL sharedTypes.URL
}

func (m *manager) SwitchLanguage(_ context.Context, request *types.SwitchLanguageRequest, response *types.SwitchLanguageResponse) error {
	if !m.allowed.Has(request.Lng) {
		return &errors.ValidationError{Msg: "invalid lng"}
	}
	request.Session.Language = request.Lng

	back, err := sharedTypes.ParseAndValidateURL(request.Referrer)
	if err != nil {
		response.Redirect = m.siteURL.String()
		return nil
	}
	if back.Host != m.siteURL.Host || back.Path == "/switch-language" {
		// No open redirect or redirect loop
		response.Redirect = m.siteURL.String()
	} else {
		response.Redirect = back.String()
	}
	return nil
}
