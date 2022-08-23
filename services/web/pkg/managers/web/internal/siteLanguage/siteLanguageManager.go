// Golang port of Overleaf
// Copyright (C) 2022 Jakob Ackermann <das7pad@outlook.com>
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
	allowed []string
	siteURL sharedTypes.URL
}

func (m *manager) SwitchLanguage(ctx context.Context, request *types.SwitchLanguageRequest, response *types.SwitchLanguageResponse) error {
	allowed := false
	for _, s := range m.allowed {
		if request.Lng == s {
			allowed = true
			break
		}
	}
	if !allowed {
		return &errors.ValidationError{Msg: "invalid lng"}
	}
	request.Session.User.Language = request.Lng
	if _, err := request.Session.Save(ctx); err != nil {
		return err
	}
	back, err := sharedTypes.ParseAndValidateURL(request.Referrer)
	if err != nil {
		return err
	}
	if back.Host == m.siteURL.Host && back.Path != "/switch-language" {
		// No open redirect or redirect loop
		response.Redirect = back.String()
	} else {
		response.Redirect = "/"
	}
	return nil
}
