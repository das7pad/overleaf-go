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
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"

	"github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
)

type Manager interface {
	GetDoc(ctx context.Context, projectId, docId primitive.ObjectID) (*types.FlushedDoc, error)
	SetDoc(ctx context.Context, projectId, docId primitive.ObjectID, doc *types.SetDocDetails) error
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
			Timeout: 5 * time.Second,
		},
	}, nil
}

const (
	maxRetries = 2
	backOff    = 5 * time.Second
)

type manager struct {
	baseURL string

	client *http.Client
}

func (m *manager) GetDoc(ctx context.Context, projectId, docId primitive.ObjectID) (*types.FlushedDoc, error) {
	var err error
	var doc *types.FlushedDoc
	for i := 0; i < maxRetries; i++ {
		if i != 0 {
			time.Sleep(backOff)
		}
		if err2 := ctx.Err(); err2 != nil {
			return nil, err2
		}
		doc, err = m.getDocOnce(ctx, projectId, docId)
		if err2 := ctx.Err(); err2 != nil {
			return nil, err2
		}
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
func (m *manager) SetDoc(ctx context.Context, projectId, docId primitive.ObjectID, doc *types.SetDocDetails) error {
	blob, err := json.Marshal(doc)
	if err != nil {
		return errors.Tag(err, "cannot serialize doc")
	}

	for i := 0; i < maxRetries; i++ {
		if i != 0 {
			time.Sleep(backOff)
		}
		if err2 := ctx.Err(); err2 != nil {
			return err2
		}
		err = m.setDocOnce(ctx, projectId, docId, blob)
		if err2 := ctx.Err(); err2 != nil {
			return err2
		}
		if err == nil {
			return nil
		}
		err = errors.Tag(err, "cannot set doc")
		if errors.IsNotFoundError(err) {
			return err
		}
	}
	return err
}

func (m *manager) setDocOnce(ctx context.Context, projectId, docId primitive.ObjectID, blob json.RawMessage) error {
	body := bytes.NewReader(blob)
	u := m.baseURL
	u += "/project/" + projectId.Hex()
	u += "/doc/" + docId.Hex()
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, u, body)
	if err != nil {
		return err
	}
	r.Header.Set("Content-Type", "application/json")
	res, err := m.client.Do(r)
	if err != nil {
		return err
	}
	_ = res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK, http.StatusAccepted, http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return &errors.NotFoundError{}
	default:
		return errors.New(
			"non-success status code from web: " + res.Status,
		)
	}
}
