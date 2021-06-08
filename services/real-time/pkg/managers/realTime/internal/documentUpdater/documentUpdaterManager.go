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

package documentUpdater

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/das7pad/real-time/pkg/errors"
	"github.com/das7pad/real-time/pkg/types"
)

type Manager interface {
	JoinDoc(ctx context.Context, client *types.Client, request *types.JoinDocRequest) (*types.JoinDocResponse, error)
}

func New(options *types.Options) (Manager, error) {
	baseURL, err := url.Parse(options.APIs.DocumentUpdater.URL)
	if err != nil {
		return nil, err
	}
	if baseURL.Scheme == "" {
		return nil, &errors.ValidationError{
			Msg: "documentUpdater URL is missing scheme",
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

func (m *manager) JoinDoc(ctx context.Context, client *types.Client, request *types.JoinDocRequest) (*types.JoinDocResponse, error) {
	u := m.baseURL
	u += "/project/" + client.ProjectId.Hex()
	u += "/doc/" + request.DocId.Hex()
	u += "?fromVersion" + strconv.FormatInt(int64(request.FromVersion), 10)
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	res, err := m.client.Do(r)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = res.Body.Close()
	}()
	switch res.StatusCode {
	case http.StatusOK:
		var body types.JoinDocResponse
		err = json.NewDecoder(res.Body).Decode(&body)
		if err != nil {
			return nil, err
		}
		return &body, nil
	case http.StatusForbidden:
		return nil, &errors.NotAuthorizedError{}
	case http.StatusNotFound, http.StatusUnprocessableEntity:
		return nil, &errors.CodedError{
			Description: "doc updater could not load requested ops",
			Code:        "DocNotFoundOrVersionTooOld",
		}
	default:
		return nil, errors.New(
			"non-success status code from document-updater: " + res.Status,
		)
	}
}
