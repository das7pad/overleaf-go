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
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	documentUpdaterTypes "github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

type Manager interface {
	GetDoc(ctx context.Context, projectId, docId primitive.ObjectID, fromVersion sharedTypes.Version) (*documentUpdaterTypes.GetDocResponse, error)
	CheckDocExists(ctx context.Context, projectId, docId primitive.ObjectID) error
	FlushProject(ctx context.Context, projectId primitive.ObjectID) error
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

func (m *manager) GetDoc(ctx context.Context, projectId, docId primitive.ObjectID, fromVersion sharedTypes.Version) (*documentUpdaterTypes.GetDocResponse, error) {
	u := m.baseURL
	u += "/project/" + projectId.Hex()
	u += "/doc/" + docId.Hex()
	u += "?fromVersion=" + fromVersion.String()
	u += "&snapshot=true"
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
		var body documentUpdaterTypes.GetDocResponse
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

func (m *manager) CheckDocExists(ctx context.Context, projectId, docId primitive.ObjectID) error {
	u := m.baseURL
	u += "/project/" + projectId.Hex()
	u += "/doc/" + docId.Hex()
	u += "/exists"
	r, err := http.NewRequestWithContext(ctx, http.MethodHead, u, nil)
	if err != nil {
		return err
	}
	res, err := m.client.Do(r)
	if err != nil {
		return err
	}
	_ = res.Body.Close()
	switch res.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusForbidden, http.StatusNotFound:
		return &errors.NotAuthorizedError{}
	default:
		return errors.New(
			"non-success status code from document-updater: " + res.Status,
		)
	}
}

func (m *manager) FlushProject(ctx context.Context, projectId primitive.ObjectID) error {
	u := m.baseURL
	u += "/project/" + projectId.Hex()
	u += "?background=true"
	r, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return err
	}
	res, err := m.client.Do(r)
	if err != nil {
		return err
	}
	_ = res.Body.Close()
	switch res.StatusCode {
	case http.StatusNoContent:
		return nil
	default:
		return errors.New(
			"non-success status code from document-updater: " + res.Status,
		)
	}
}
