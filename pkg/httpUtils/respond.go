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

package httpUtils

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/das7pad/overleaf-go/pkg/asyncForm"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/httpTiming"
	"github.com/das7pad/overleaf-go/pkg/session"
	"github.com/das7pad/overleaf-go/services/web/pkg/templates"
)

func RespondErr(c *gin.Context, err error) {
	Respond(c, 0, nil, err)
}

func Respond(
	c *gin.Context,
	code int,
	body interface{},
	err error,
) {
	if err != nil {
		var errMessage string
		code, errMessage = getAndLogErrResponseDetails(c, err)
		if body == nil {
			body = gin.H{"message": errMessage}
		} else if r, ok := body.(*asyncForm.Response); ok && r.Message == nil {
			r.Message = &asyncForm.Message{
				Text: errMessage,
				Type: asyncForm.Error,
			}
		}
	}
	httpTiming.EndTotalTimer(c)
	c.Abort()
	if body == nil {
		c.Status(code)
	} else {
		c.JSON(code, body)
	}
}

func RespondHTML(
	c *gin.Context,
	body templates.Renderer,
	err error,
	s *session.User,
	ps *templates.PublicSettings,
) {
	code := http.StatusOK
	if err != nil {
		var errMessage string
		code, errMessage = getAndLogErrResponseDetails(c, err)
		switch code {
		case 400:
			body = &templates.General400Data{
				NoJsLayoutData: templates.NoJsLayoutData{
					CommonData: templates.CommonData{
						Settings:              ps,
						RobotsNoindexNofollow: true,
						SessionUser:           s,
						Title:                 "",
						TitleLocale:           "",
						Viewport:              false,
					},
				},
				Message: errMessage,
			}
		case 404:
			body = &templates.General404Data{
				MarketingLayoutData: templates.MarketingLayoutData{
					JsLayoutData: templates.JsLayoutData{
						CommonData: templates.CommonData{
							Settings:              ps,
							RobotsNoindexNofollow: true,
							SessionUser:           s,
							Title:                 "",
							TitleLocale:           "",
							Viewport:              false,
						},
					},
				},
			}
		default:
			body = &templates.General500Data{
				NoJsLayoutData: templates.NoJsLayoutData{
					CommonData: templates.CommonData{
						Settings:              ps,
						RobotsNoindexNofollow: true,
						SessionUser:           s,
						Title:                 "",
						TitleLocale:           "",
						Viewport:              false,
					},
				},
			}
		}
	}
	var blob string
	blob, err = body.Render()
	if err != nil {
		err = errors.Tag(err, "cannot render")
		if code == 500 {
			c.Abort()
			httpTiming.EndTotalTimer(c)
			getAndLogErrResponseDetails(c, err)
			c.String(500, "internal render error")
			return
		}
		RespondHTML(c, body, err, s, ps)
		return
	}
	c.Abort()
	httpTiming.EndTotalTimer(c)
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(code, blob)
}

func getAndLogErrResponseDetails(c *gin.Context, err error) (int, string) {
	code := 500
	errMessage := err.Error()
	if errors.IsValidationError(err) {
		code = http.StatusBadRequest
	} else if errors.IsUnauthorizedError(err) {
		code = http.StatusUnauthorized
	} else if errors.IsNotAuthorizedError(err) {
		code = http.StatusForbidden
	} else if errors.IsDocNotFoundError(err) {
		code = http.StatusNotFound
	} else if errors.IsMissingOutputFileError(err) {
		code = http.StatusNotFound
	} else if errors.IsNotFoundError(err) {
		code = http.StatusNotFound
	} else if errors.IsInvalidState(err) {
		code = http.StatusConflict
	} else if errors.IsBodyTooLargeError(err) {
		code = http.StatusRequestEntityTooLarge
	} else if errors.IsUnprocessableEntity(err) {
		code = http.StatusUnprocessableEntity
	} else if errors.IsUpdateRangeNotAvailableError(err) {
		code = http.StatusUnprocessableEntity
	} else if errors.IsAlreadyCompiling(err) {
		code = http.StatusLocked
	} else {
		log.Printf(
			"%s %s: %s",
			c.Request.Method, c.Request.URL.Path, errMessage,
		)
		code = http.StatusInternalServerError
		errMessage = "internal server error"
	}
	return code, errMessage
}
