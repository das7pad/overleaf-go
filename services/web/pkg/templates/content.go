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
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type PublicSettings struct {
	AppName    string
	AdminEmail sharedTypes.Email
	I18n       struct {
		SubdomainLang []struct {
			Hide    bool
			LngCode string
			URL     sharedTypes.URL
		}
	}
	Nav struct {
		HeaderExtras []struct {
			Dropdown []struct {
				Divider bool
				Class   string
				Label   string
				Text    string
				URL     string
			}
		}
		LeftFooter []struct {
			Class string
			Label string
			Text  string
			URL   string
		}
		RightFooter []struct {
			Class string
			Label string
			Text  string
			URL   string
		}
	}
	RobotsNoindex       bool
	TranslatedLanguages map[string]string
}

func (s *PublicSettings) ShowLanguagePicker() bool {
	return len(s.I18n.SubdomainLang) > 1
}

type CommonMetadata struct {
	CurrentLngCode        string
	RobotsNoindexNofollow bool
	Title                 string
	Viewport              bool
}

type MarketingLayoutData struct {
	*PublicSettings
	CommonMetadata

	UsersEmail sharedTypes.Email
	UserId     primitive.ObjectID
	IsAdmin    bool
}

func (d *MarketingLayoutData) LoggedIn() bool {
	return !d.UserId.IsZero()
}

func (d *MarketingLayoutData) UserIdNoZero() string {
	if d.UserId.IsZero() {
		return ""
	}
	return d.UserId.Hex()
}
