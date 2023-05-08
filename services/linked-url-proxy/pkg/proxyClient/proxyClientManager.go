// Golang port of Overleaf
// Copyright (C) 2022-2023 Jakob Ackermann <das7pad@outlook.com>
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

package proxyClient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/linked-url-proxy/pkg/constants"
)

type Manager interface {
	Fetch(ctx context.Context, src *sharedTypes.URL) (io.ReadCloser, func(), error)
}

func New(chain []sharedTypes.URL) (Manager, error) {
	if len(chain) < 1 {
		return nil, &errors.ValidationError{Msg: "url chain is too short"}
	}
	return &manager{
		chain: chain,
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return errors.New("blocked redirect")
			},
		},
	}, nil
}

type manager struct {
	chain  []sharedTypes.URL
	client *http.Client
}

func (m *manager) Fetch(ctx context.Context, src *sharedTypes.URL) (io.ReadCloser, func(), error) {
	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, chainURL(src, m.chain), nil,
	)
	if err != nil {
		return nil, nil, errors.Tag(err, "prepare http request")
	}
	res, err := m.client.Do(req)
	if err != nil {
		return nil, nil, errors.Tag(err, "send http request")
	}
	cleanup := func() {
		// Just close the body. In case the body has not been consumed in full
		//  yet, the connection cannot be re-used. This is fine and better than
		//  consuming up-to 50MB of useless bandwidth.
		_ = res.Body.Close()
	}
	if res.StatusCode != http.StatusOK {
		defer cleanup()
		switch res.StatusCode {
		case http.StatusBadRequest:
			msg := map[string]string{}
			if err = json.NewDecoder(res.Body).Decode(&msg); err != nil {
				return nil, nil, errors.Tag(err, "decode 400 response")
			}
			return nil, nil, &errors.ValidationError{Msg: msg["message"]}
		case http.StatusUnprocessableEntity:
			return nil, nil, &errors.UnprocessableEntityError{
				Msg: fmt.Sprintf(
					"upstream returned non success: %s",
					res.Header.Get(constants.HeaderXUpstreamStatusCode),
				),
			}
		case http.StatusRequestEntityTooLarge:
			return nil, nil, &errors.BodyTooLargeError{}
		default:
			return nil, nil, errors.New(fmt.Sprintf(
				"proxy returned non success: %d", res.StatusCode,
			))
		}
	}
	return res.Body, cleanup, nil
}
