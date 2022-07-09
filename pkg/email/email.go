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

package email

import (
	"io"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type Email struct {
	Content Content
	ReplyTo Identity
	Subject string
	To      Identity
}

func (e *Email) Validate() error {
	if err := e.Content.Validate(); err != nil {
		return errors.New("invalid content: " + err.Error())
	}
	if e.ReplyTo.Address != "" {
		if err := e.ReplyTo.Validate(); err != nil {
			return errors.New("invalid recipient: " + err.Error())
		}
	}
	if len(e.Subject) == 0 {
		return errors.New("missing subject")
	}
	if err := e.To.Validate(); err != nil {
		return errors.New("invalid recipient: " + err.Error())
	}
	return nil
}

func (e *Email) writeHTML(w io.Writer) error {
	return e.Content.Template().Execute(w, e.Content)
}

func (e *Email) writePlainText(w io.Writer) error {
	_, err := io.WriteString(w, e.Content.PlainText())
	return err
}
