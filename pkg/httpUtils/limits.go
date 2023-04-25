// Golang port of Overleaf
// Copyright (C) 2023 Jakob Ackermann <das7pad@outlook.com>
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

package httpUtils

import (
	"github.com/das7pad/overleaf-go/pkg/constants"
	"github.com/das7pad/overleaf-go/pkg/errors"
)

func validateContentLength(c *Context) error {
	if cl := c.Request.ContentLength; cl == -1 {
		return &errors.UnprocessableEntityError{
			Msg: "missing Content-Length header",
		}
	} else if cl > constants.MaxUploadSize {
		return &errors.BodyTooLargeError{}
	}
	return nil
}
