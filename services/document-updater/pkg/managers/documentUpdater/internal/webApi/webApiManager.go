// Golang port of the Overleaf document-updater service
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
	"net/url"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/document-updater/pkg/errors"
	"github.com/das7pad/document-updater/pkg/types"
)

type Manager interface {
	GetDoc(ctx context.Context, projectId, docId primitive.ObjectID) (*types.FlushedDoc, error)
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

func (m *manager) GetDoc(ctx context.Context, projectId, docId primitive.ObjectID) (*types.FlushedDoc, error) {
	var err error
	var doc *types.FlushedDoc
	for i := 0; i < 2; i++ {
		if i != 0 {
			time.Sleep(10 * time.Second)
		}
		doc, err = m.getDocOnce(ctx, projectId, docId)
		if err == nil {
			return doc, nil
		}
		err = errors.Tag(err, "cannot get doc")
		if errors.IsNotFoundError(err) || errors.IsNotAuthorizedError(err) {
			return nil, err
		}
	}
	return nil, err
}

func (m *manager) getDocOnce(ctx context.Context, projectId, docId primitive.ObjectID) (*types.FlushedDoc, error) {
	u := m.baseURL
	u += "/project/" + projectId.Hex()
	u += "/doc/" + docId.Hex()
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
		var body types.FlushedDoc
		err = json.NewDecoder(res.Body).Decode(&body)
		if err != nil {
			return nil, err
		}
		return &body, nil
	case http.StatusForbidden:
		return nil, &errors.NotAuthorizedError{}
	case http.StatusNotFound:
		return nil, &errors.NotFoundError{}
	default:
		return nil, errors.New(
			"non-success status code from web: " + res.Status,
		)
	}
}
