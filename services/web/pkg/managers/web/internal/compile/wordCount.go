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

package compile

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/das7pad/overleaf-go/pkg/errors"
	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) WordCount(ctx context.Context, request *types.WordCountRequest, words *clsiTypes.Words) error {
	u := m.baseURL
	u += "/project/" + request.ProjectId.Hex()
	u += "/user/" + request.UserId.Hex()
	u += "/wordcount"

	request.WordCountRequest.CompileGroup =
		request.SignedCompileProjectRequestOptions.CompileGroup
	request.ImageName = m.getImageName(request.ImageName)

	blob, err := json.Marshal(request.WordCountRequest)
	if err != nil {
		return errors.Tag(err, "cannot serialize word count request")
	}
	body := bytes.NewReader(blob)

	r, err := http.NewRequestWithContext(ctx, http.MethodPost, u, body)
	if err != nil {
		return errors.Tag(err, "cannot create word count request")
	}
	res, err := m.doStaticRequest(request.ClsiServerId, r)
	if err != nil {
		return errors.Tag(err, "cannot action word count request")
	}
	defer func() {
		_ = res.Body.Close()
	}()

	switch res.StatusCode {
	case http.StatusOK:
		return json.NewDecoder(res.Body).Decode(words)
	default:
		return unexpectedStatus(res)
	}
}
