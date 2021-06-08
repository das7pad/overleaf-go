// Golang port of the Overleaf real-time service
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

package webApiManager

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/das7pad/real-time/pkg/errors"
	"github.com/das7pad/real-time/pkg/types"
)

type Manager interface {
	JoinProject(ctx context.Context, client *types.Client, request *types.JoinProjectRequest) (*types.JoinProjectWebApiResponse, error)
}

func New(options *types.Options) (Manager, error) {
	baseURL, err := url.Parse(options.APIs.WebApi.URL)
	if err != nil {
		return nil, err
	}
	if baseURL.Scheme == "" {
		return nil, &errors.ValidationError{
			Msg: "webApi URL is missing scheme",
		}
	}
	return &manager{
		baseURL: baseURL.String(),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

type manager struct {
	baseURL string

	client *http.Client
}

const (
	anonymousAccessTokenHeader = "x-sl-anonymous-access-token"
)

func (m *manager) JoinProject(ctx context.Context, client *types.Client, request *types.JoinProjectRequest) (*types.JoinProjectWebApiResponse, error) {
	u := m.baseURL
	u += "/project/" + request.ProjectId.Hex() + "/join"
	u += "?user_id=" + client.User.Id.Hex()
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	if err != nil {
		return nil, err
	}
	r.Header.Set(anonymousAccessTokenHeader, request.AnonymousAccessToken)
	res, err := m.client.Do(r)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = res.Body.Close()
	}()
	switch res.StatusCode {
	case http.StatusOK:
		var body types.JoinProjectWebApiResponse
		err = json.NewDecoder(res.Body).Decode(&body)
		if err != nil {
			return nil, err
		}
		return &body, nil
	case http.StatusForbidden:
		return nil, &errors.NotAuthorizedError{}
	case http.StatusNotFound:
		return nil, &errors.CodedError{
			Description: "project not found",
			Code:        "ProjectNotFound",
		}
	case http.StatusTooManyRequests:
		return nil, &errors.CodedError{
			Description: "rate-limit hit when joining project",
			Code:        "TooManyRequests",
		}
	default:
		return nil, errors.New(
			"non-success status code from web: " + res.Status,
		)
	}
}
