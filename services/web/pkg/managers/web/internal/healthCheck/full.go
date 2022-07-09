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

package healthCheck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/das7pad/overleaf-go/pkg/asyncForm"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type smokeTestSession struct {
	baseURL            string
	client             *http.Client
	loginBody          json.RawMessage
	projectIdHex       string
	regexProjectIdMeta *regexp.Regexp
}

type failure struct {
	Msg string
}

func (e *failure) Error() string {
	return e.Msg
}

func expectAsyncFormWithRedirect(res *http.Response, to string) error {
	defer func() {
		_ = res.Body.Close()
	}()
	if res.StatusCode != http.StatusOK {
		return &failure{
			Msg: fmt.Sprintf("non success status %d", res.StatusCode),
		}
	}
	b := asyncForm.Response{}
	if err := json.NewDecoder(res.Body).Decode(&b); err != nil {
		return errors.Tag(err, "cannot decode body")
	}
	if b.Message != nil {
		return errors.Tag(
			&failure{Msg: b.Message.Text},
			"server error message",
		)
	}
	if b.RedirectTo != to {
		return &failure{
			Msg: fmt.Sprintf("redirect not %s: %q", to, b.RedirectTo),
		}
	}
	return nil
}

func (s *smokeTestSession) request(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
	r, err := http.NewRequestWithContext(ctx, method, s.baseURL+url, body)
	if err != nil {
		return nil, errors.Tag(err, "cannot compose request")
	}
	r.Header.Set("X-Forwarded-Proto", "https")
	res, err := s.client.Do(r)
	if err != nil {
		return nil, errors.Tag(err, "cannot send request")
	}
	if res.StatusCode != http.StatusOK {
		_ = res.Body.Close()
		return nil, &failure{
			Msg: fmt.Sprintf("non success status %d", res.StatusCode),
		}
	}
	return res, nil
}

func (s *smokeTestSession) html(ctx context.Context, url string) ([]byte, error) {
	res, err := s.request(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = res.Body.Close()
	}()
	blob, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Tag(err, "cannot consume body")
	}
	return blob, nil
}

func (s *smokeTestSession) apiCall(ctx context.Context, url string, body io.Reader, redirectTo string) error {
	res, err := s.request(ctx, http.MethodPost, url, body)
	if err != nil {
		return err
	}
	return expectAsyncFormWithRedirect(res, redirectTo)
}

func (s *smokeTestSession) login(ctx context.Context) error {
	return s.apiCall(
		ctx, "/api/login", bytes.NewBuffer(s.loginBody), "/project",
	)
}

func (s *smokeTestSession) logout(ctx context.Context) error {
	return s.apiCall(
		ctx, "/api/logout", nil, "/login",
	)
}

var (
	regexProjectListTitle      = regexp.MustCompile("<title>\\s*Your Projects - .*,\\s*Online LaTeX Editor\\s*</title>")
	regexProjectListController = regexp.MustCompile(`controller="ProjectPageController"`)
)

func (s *smokeTestSession) projectList(ctx context.Context) error {
	blob, err := s.html(ctx, "/project")
	if err != nil {
		return err
	}
	if !regexProjectListTitle.Match(blob) {
		return &failure{Msg: "mismatching title"}
	}
	if !regexProjectListController.Match(blob) {
		return &failure{Msg: "missing angular controller"}
	}
	return nil
}

func (s *smokeTestSession) projectEditor(ctx context.Context) error {
	blob, err := s.html(ctx, "/project/"+s.projectIdHex)
	if err != nil {
		return err
	}
	if !s.regexProjectIdMeta.Match(blob) {
		return &failure{Msg: "missing project_id meta tag"}
	}
	return nil
}

type insecureJar struct {
	cookies []*http.Cookie
}

func (i *insecureJar) SetCookies(_ *url.URL, cookies []*http.Cookie) {
	i.cookies = cookies
}

func (i *insecureJar) Cookies(_ *url.URL) []*http.Cookie {
	return i.cookies
}

func (m *manager) SmokeTestFull(ctx context.Context, response *types.SmokeTestResponse) error {
	start := time.Now()

	s := smokeTestSession{
		baseURL: m.smokeTestBaseURL,
		client: &http.Client{
			Timeout: 3 * time.Second,
			Jar:     &insecureJar{},
		},
		loginBody:          m.smokeTestLoginBody,
		projectIdHex:       m.smokeTestProjectIdHex,
		regexProjectIdMeta: m.smokeTestProjectIdMeta,
	}
	steps := []*types.SmokeTestStep{
		{
			Name: "init",
		},
		{
			Name:   "login",
			Action: s.login,
		},
		{
			Name:   "projectList",
			Action: s.projectList,
		},
		{
			Name:   "projectEditor",
			Action: s.projectEditor,
		},
		{
			Name:   "logout",
			Action: s.logout,
		},
	}
	var fatalError error
	lastEnd := start
	for _, step := range steps {
		if action := step.Action; action != nil {
			err := action(ctx)
			if err != nil {
				fatalError = errors.Tag(err, step.Name)
			}
		}
		now := time.Now()
		step.Duration = now.Sub(lastEnd).String()
		lastEnd = now
		if fatalError != nil {
			break
		}
	}
	if fatalError != nil {
		cCtx, done := context.WithTimeout(context.Background(), 3*time.Second)
		_ = s.logout(cCtx)
		done()
	}
	response.Stats = &types.SmokeTestStats{
		Start:    start,
		Steps:    steps,
		End:      lastEnd,
		Duration: lastEnd.Sub(start).String(),
	}
	return fatalError
}
