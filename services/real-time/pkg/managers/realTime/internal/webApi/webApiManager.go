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

package webApi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

type Manager interface {
	JoinProject(ctx context.Context, client *types.Client, request *types.JoinProjectRequest) (*types.JoinProjectWebApiResponse, string, error)
}

func New(options *types.Options, db *mongo.Database) (Manager, error) {
	if options.APIs.WebApi.Monolith {
		pm := project.New(db)
		um := user.New(db)
		return &monolithManager{
			pm: pm,
			um: um,
		}, nil
	}
	return &manager{
		baseURL: options.APIs.WebApi.URL.String(),
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

func (m *manager) JoinProject(ctx context.Context, client *types.Client, request *types.JoinProjectRequest) (*types.JoinProjectWebApiResponse, string, error) {
	u := m.baseURL
	u += "/project/" + request.ProjectId.Hex() + "/join"
	u += "?user_id=" + client.User.Id.Hex()
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	if err != nil {
		return nil, "", err
	}
	r.Header.Set(anonymousAccessTokenHeader, string(request.AnonymousAccessToken))
	res, err := m.client.Do(r)
	if err != nil {
		return nil, "", err
	}
	defer func() {
		_ = res.Body.Close()
	}()
	by := res.Header.Get("X-Served-By")
	switch res.StatusCode {
	case http.StatusOK:
		var body types.JoinProjectWebApiResponse
		err = json.NewDecoder(res.Body).Decode(&body)
		if err != nil {
			return nil, by, err
		}
		return &body, by, nil
	case http.StatusForbidden:
		return nil, by, &errors.NotAuthorizedError{}
	case http.StatusNotFound:
		return nil, by, &errors.CodedError{
			Description: "project not found",
			Code:        "ProjectNotFound",
		}
	case http.StatusTooManyRequests:
		return nil, by, &errors.CodedError{
			Description: "rate-limit hit when joining project",
			Code:        "TooManyRequests",
		}
	default:
		return nil, by, errors.New(
			"non-success status code from web: " + res.Status,
		)
	}
}
