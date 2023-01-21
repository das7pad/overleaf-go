// Golang port of Overleaf
// Copyright (C) 2021-2023 Jakob Ackermann <das7pad@outlook.com>
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
	"strconv"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/session"
)

func shouldRedirectToLogin(s *session.Session, err error) bool {
	if !errors.IsUnauthorizedError(err) {
		return false
	}
	if s.IsLoggedIn() {
		return false
	}
	return true
}

func RespondHTML(
	c *httpUtils.Context,
	body Renderer,
	err error,
	s *session.Session,
	ps *PublicSettings,
	flushSession func(c *httpUtils.Context, session *session.Session) error,
) {
	RespondHTMLCustomStatus(c, http.StatusOK, body, err, s, ps, flushSession)
}

func RespondHTMLCustomStatus(
	c *httpUtils.Context,
	code int,
	body Renderer,
	err error,
	s *session.Session,
	ps *PublicSettings,
	flushSession func(c *httpUtils.Context, session *session.Session) error,
) {
	if err != nil {
		if shouldRedirectToLogin(s, err) {
			if c.Request.URL.Path != "/project" {
				// We are redirecting to /project after login by default.
				s.PostLoginRedirect = ps.SiteURL.
					WithPath(c.Request.URL.Path).
					WithQuery(c.Request.URL.Query()).
					String()
				_ = flushSession(c, s)
			}
			q := c.Request.URL.Query()
			if q.Get("project_name") != "" {
				// Show SharedProjectData details on registration page.
				httpUtils.Redirect(c, ps.SiteURL.
					WithPath("/register").
					WithQuery(q).
					String(),
				)
			} else {
				httpUtils.Redirect(c, "/login")
			}
			return
		}
		var errMessage string
		code, errMessage = httpUtils.GetAndLogErrResponseDetails(c, err)
		switch code {
		case http.StatusBadRequest,
			http.StatusConflict,
			http.StatusUnprocessableEntity,
			http.StatusTooManyRequests:
			body = &General400Data{
				NoJsLayoutData: NoJsLayoutData{
					CommonData: CommonData{
						Settings:              ps,
						RobotsNoindexNofollow: true,
						Session:               s.PublicData,
						Title:                 "Client Error",
						Viewport:              true,
					},
				},
				Message: errMessage,
			}
		case http.StatusUnauthorized, http.StatusForbidden:
			body = &UserRestrictedData{
				MarketingLayoutData: MarketingLayoutData{
					CommonData: CommonData{
						Settings:              ps,
						RobotsNoindexNofollow: true,
						Session:               s.PublicData,
						TitleLocale:           "restricted",
						Viewport:              true,
					},
				},
			}
		case http.StatusNotFound:
			body = &General404Data{
				MarketingLayoutData: MarketingLayoutData{
					CommonData: CommonData{
						Settings:              ps,
						RobotsNoindexNofollow: true,
						Session:               s.PublicData,
						TitleLocale:           "page_not_found",
						Viewport:              true,
					},
				},
			}
		default:
			body = &General500Data{
				NoJsLayoutData: NoJsLayoutData{
					CommonData: CommonData{
						Settings:              ps,
						RobotsNoindexNofollow: true,
						Session:               s.PublicData,
						TitleLocale:           "server_error",
						Viewport:              true,
					},
				},
			}
		}
	}
	var blob []byte
	var hints string
	doneRender := httpUtils.TimeStage(c, "render")
	blob, hints, err = body.Render()
	doneRender()
	if err != nil {
		if code == http.StatusInternalServerError {
			httpUtils.GetAndLogErrResponseDetails(c, err)
			httpUtils.RespondPlain(
				c, http.StatusInternalServerError, "internal render error",
			)
			return
		}
		RespondHTML(c, body, err, s, ps, flushSession)
		return
	}
	h := c.Writer.Header()
	h.Set("Content-Length", strconv.FormatInt(int64(len(blob)), 10))
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Security-Policy", body.CSP())
	h.Set("Link", hints)
	httpUtils.EndTotalTimer(c)
	c.Writer.WriteHeader(code)
	_, _ = c.Writer.Write(blob)
}
