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

package email

import (
	"html/template"

	"github.com/das7pad/overleaf-go/pkg/email/internal/templates"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type HelpLink struct {
	Before string
	URL    *sharedTypes.URL
	Label  string
	After  string
}

type NoCTAContent struct {
	*PublicOptions

	Message   Message
	Title     string
	HelpLinks []HelpLink
}

func (c *NoCTAContent) Validate() error {
	if len(c.Message) == 0 && len(c.HelpLinks) == 0 {
		return errors.New("missing Message")
	}
	if len(c.Title) == 0 {
		return errors.New("missing Title")
	}
	return nil
}

func (c *NoCTAContent) Template() *template.Template {
	return templates.NoCTA
}

func (c *NoCTAContent) PlainText() string {
	helpLinks := ""
	if len(c.HelpLinks) > 0 {
		for _, helpLink := range c.HelpLinks {
			helpLinks += "\n" +
				helpLink.Before +
				helpLink.Label + " (" + helpLink.URL.String() + ")" +
				helpLink.After + "\n"
		}
	}
	s := "Hi," + "\n" +
		"\n" +
		c.Message.String() + "\n" +
		helpLinks +
		"\n" +
		"Regards," + "\n" +
		"The " + c.AppName + " Team - " + c.SiteURL

	if c.CustomFooter != "" {
		s += "\n\n" + c.CustomFooter
	}
	return s
}
