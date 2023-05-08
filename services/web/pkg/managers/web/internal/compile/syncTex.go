// Golang port of Overleaf
// Copyright (C) 2021-2023 Jakob Ackermann <das7pad@outlook.com>
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
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type genericPOSTRequest interface {
	SetCompileGroup(sharedTypes.CompileGroup)
	SetImageName(sharedTypes.ImageName)
	Validate() error
}

func (m *manager) preprocessGenericPOST(options sharedTypes.SignedCompileProjectRequestOptions, imageName sharedTypes.ImageName, request genericPOSTRequest) error {
	request.SetCompileGroup(options.CompileGroup)
	if err := request.Validate(); err != nil {
		return err
	}
	request.SetImageName(m.getImageName(imageName))
	return nil
}

func (m *manager) genericPOST(ctx context.Context, endpoint string, options sharedTypes.SignedCompileProjectRequestOptions, clsiServerId types.ClsiServerId, request genericPOSTRequest, response interface{}) error {
	u := m.getURL(options.ProjectId, options.UserId, endpoint)

	blob, err := json.Marshal(request)
	if err != nil {
		return errors.Tag(err, "serialize request")
	}
	body := bytes.NewReader(blob)

	r, err := http.NewRequestWithContext(ctx, http.MethodPost, u, body)
	if err != nil {
		return errors.Tag(err, "create request")
	}
	res, err := m.doStaticRequest(clsiServerId, r)
	if err != nil {
		return errors.Tag(err, "action request")
	}
	defer func() {
		_ = res.Body.Close()
	}()

	switch res.StatusCode {
	case http.StatusOK:
		return json.NewDecoder(res.Body).Decode(response)
	default:
		return unexpectedStatus(res)
	}
}

func (m *manager) SyncFromCode(ctx context.Context, request *types.SyncFromCodeRequest, response *types.SyncFromCodeResponse) error {
	err := m.preprocessGenericPOST(
		request.SignedCompileProjectRequestOptions,
		request.ImageName,
		request,
	)
	if err != nil {
		return err
	}
	if m.bundle != nil {
		return m.bundle.SyncFromCode(
			ctx, request.ProjectId, request.UserId,
			&request.SyncFromCodeRequest, response,
		)
	}
	return m.genericPOST(
		ctx,
		"/sync/code",
		request.SignedCompileProjectRequestOptions,
		request.ClsiServerId,
		&request.SyncFromCodeRequest,
		response,
	)
}

func (m *manager) SyncFromPDF(ctx context.Context, request *types.SyncFromPDFRequest, response *types.SyncFromPDFResponse) error {
	err := m.preprocessGenericPOST(
		request.SignedCompileProjectRequestOptions,
		request.ImageName,
		request,
	)
	if err != nil {
		return err
	}
	if m.bundle != nil {
		return m.bundle.SyncFromPDF(
			ctx, request.ProjectId, request.UserId,
			&request.SyncFromPDFRequest, response,
		)
	}
	return m.genericPOST(
		ctx,
		"/sync/pdf",
		request.SignedCompileProjectRequestOptions,
		request.ClsiServerId,
		&request.SyncFromPDFRequest,
		response,
	)
}
