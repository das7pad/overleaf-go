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

package editor

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/jwt/jwtHandler"
	"github.com/das7pad/overleaf-go/pkg/jwt/userIdJWT"
	"github.com/das7pad/overleaf-go/pkg/jwt/wsBootstrap"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/managers/docstore"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	LoadEditor(ctx context.Context, request *types.LoadEditorRequest, response *types.LoadEditorResponse) error
	GetProjectJWT(ctx context.Context, request *types.GetProjectJWTRequest, response *types.GetProjectJWTResponse) error
}

func New(options *types.Options, pm project.Manager, um user.Manager, dm docstore.Manager, projectJWTHandler jwtHandler.JWTHandler, loggedInUserJWTHandler jwtHandler.JWTHandler) Manager {
	return &manager{
		dm:              dm,
		jwtProject:      projectJWTHandler,
		jwtLoggedInUser: loggedInUserJWTHandler,
		jwtSpelling:     userIdJWT.New(options.JWT.Spelling),
		pm:              pm,
		um:              um,
		wsBootstrap:     wsBootstrap.New(options.JWT.RealTime),
	}
}

type manager struct {
	dm              docstore.Manager
	jwtProject      jwtHandler.JWTHandler
	jwtLoggedInUser jwtHandler.JWTHandler
	jwtSpelling     jwtHandler.JWTHandler
	pm              project.Manager
	um              user.Manager
	wsBootstrap     jwtHandler.JWTHandler
}
