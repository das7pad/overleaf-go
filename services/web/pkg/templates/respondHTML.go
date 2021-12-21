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
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/session"
)

const (
	timingKeyRender = "templates.timing.render"
)

var (
	timerStartRender = httpUtils.StartTimer(timingKeyRender)
	timerEndRender   = httpUtils.EndTimer(timingKeyRender, "render")
)

func shouldRedirectToLogin(c *gin.Context, s *session.Session, err error) bool {
	if c.Request.Method != http.MethodGet {
		return false
	}
	if !errors.IsUnauthorizedError(err) {
		return false
	}
	if s.IsLoggedIn() {
		return false
	}
	if c.NegotiateFormat("application/json", "*") != "*" {
		return false
	}
	return true
}

func RespondHTML(
	c *gin.Context,
	body Renderer,
	err error,
	s *session.Session,
	ps *PublicSettings,
	flushSession func(c *gin.Context, session *session.Session) error,
) {
	c.Abort()
	code := http.StatusOK
	if err != nil {
		if shouldRedirectToLogin(c, s, err) {
			s.PostLoginRedirect = ps.SiteURL.
				WithPath(c.Request.URL.Path).
				WithQuery(c.Request.URL.Query()).
				String()
			_ = flushSession(c, s)
			httpUtils.EndTotalTimer(c)
			c.Redirect(http.StatusFound, "/login")
			return
		}
		var errMessage string
		code, errMessage = httpUtils.GetAndLogErrResponseDetails(c, err)
		switch code {
		case http.StatusBadRequest, http.StatusConflict, http.StatusUnprocessableEntity:
			body = &General400Data{
				NoJsLayoutData: NoJsLayoutData{
					CommonData: CommonData{
						Settings:              ps,
						RobotsNoindexNofollow: true,
						SessionUser:           s.User,
						Title:                 "Client Error",
						Viewport:              true,
					},
				},
				Message: errMessage,
			}
		case http.StatusUnauthorized, http.StatusForbidden:
			body = &UserRestrictedData{
				MarketingLayoutData: MarketingLayoutData{
					JsLayoutData: JsLayoutData{
						CommonData: CommonData{
							Settings:              ps,
							RobotsNoindexNofollow: true,
							SessionUser:           s.User,
							TitleLocale:           "restricted",
						},
					},
				},
			}
		case http.StatusNotFound:
			body = &General404Data{
				MarketingLayoutData: MarketingLayoutData{
					JsLayoutData: JsLayoutData{
						CommonData: CommonData{
							Settings:              ps,
							RobotsNoindexNofollow: true,
							SessionUser:           s.User,
							TitleLocale:           "page_not_found",
						},
					},
				},
			}
		default:
			body = &General500Data{
				NoJsLayoutData: NoJsLayoutData{
					CommonData: CommonData{
						Settings:              ps,
						RobotsNoindexNofollow: true,
						SessionUser:           s.User,
						TitleLocale:           "server_error",
						Viewport:              false,
					},
				},
			}
		}
	}
	var blob string
	timerStartRender(c)
	blob, err = body.Render()
	timerEndRender(c)
	if err != nil {
		if code == 500 {
			httpUtils.EndTotalTimer(c)
			httpUtils.GetAndLogErrResponseDetails(c, err)
			c.String(500, "internal render error")
			return
		}
		RespondHTML(c, body, err, s, ps, flushSession)
		return
	}
	httpUtils.EndTotalTimer(c)
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(code, blob)
}
