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

package linkedURLProxy

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func CleanupResponseBody(body io.ReadCloser) {
	// Consume the body to enable connection re-use.
	_, _ = io.Discard.(io.ReaderFrom).ReadFrom(body)
	_ = body.Close()
}

func (m *manager) Fetch(ctx context.Context, src *sharedTypes.URL) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, chainURL(src, m.chain), nil,
	)
	if err != nil {
		return nil, errors.Tag(err, "cannot prepare http request")
	}
	res, err := m.client.Do(req)
	if err != nil {
		return nil, errors.Tag(err, "cannot send http request")
	}
	if res.StatusCode != 200 {
		CleanupResponseBody(res.Body)
		switch res.StatusCode {
		case http.StatusUnprocessableEntity:
			return nil, &errors.UnprocessableEntityError{
				Msg: fmt.Sprintf(
					"upstream returned non success: %s",
					res.Header.Get("X-Upstream-Status-Code"),
				),
			}
		case http.StatusRequestEntityTooLarge:
			return nil, &errors.BodyTooLargeError{}
		default:
			return nil, errors.New(fmt.Sprintf(
				"proxy returned non success: %d", res.StatusCode,
			))
		}
	}
	return res.Body, nil
}
