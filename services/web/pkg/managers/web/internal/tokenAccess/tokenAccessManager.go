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

package tokenAccess

import (
	"context"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/jwt/projectJWT"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	GrantTokenAccessReadAndWrite(ctx context.Context, request *types.GrantTokenAccessRequest, response *types.GrantTokenAccessResponse) error
	GrantTokenAccessReadOnly(ctx context.Context, request *types.GrantTokenAccessRequest, response *types.GrantTokenAccessResponse) error
}

func New(client redis.UniversalClient, pm project.Manager) Manager {
	return &manager{
		client: client,
		pm:     pm,
	}
}

type manager struct {
	client redis.UniversalClient
	pm     project.Manager
}

func (m *manager) GrantTokenAccessReadAndWrite(ctx context.Context, request *types.GrantTokenAccessRequest, response *types.GrantTokenAccessResponse) error {
	return m.grantTokenAccess(
		ctx, request, response,
		m.pm.GetProjectAccessForReadAndWriteToken, m.pm.GrantReadAndWriteTokenAccess,
	)
}

func (m *manager) GrantTokenAccessReadOnly(ctx context.Context, request *types.GrantTokenAccessRequest, response *types.GrantTokenAccessResponse) error {
	return m.grantTokenAccess(
		ctx, request, response,
		m.pm.GetProjectAccessForReadOnlyToken, m.pm.GrantReadOnlyTokenAccess,
	)
}

type getTokenAccess func(ctx context.Context, userId primitive.ObjectID, token project.AccessToken) (*project.TokenAccessResult, error)
type grantAccess func(ctx context.Context, projectId primitive.ObjectID, epoch int64, userId primitive.ObjectID) error

func (m *manager) grantTokenAccess(ctx context.Context, request *types.GrantTokenAccessRequest, response *types.GrantTokenAccessResponse, getter getTokenAccess, granter grantAccess) error {
	userId := request.Session.User.Id
	token := request.Token
	for i := 0; i < 10; i++ {
		r, err := getter(ctx, userId, token)
		if err != nil {
			if errors.IsNotAuthorizedError(err) {
				response.RedirectTo = "/restricted"
				return nil
			}
			return err
		}
		projectId := r.ProjectId
		if request.Session.IsLoggedIn() {
			if r.ShouldGrantHigherAccess() {
				// Clearing the epoch is OK to do at any time.
				// Clear it just ahead of committing the tx.
				err = projectJWT.ClearProjectField(ctx, m.client, projectId)
				if err != nil {
					return err
				}
				err = granter(ctx, projectId, r.Epoch, userId)
				// Clear it again after potentially updating the epoch.
				_ = projectJWT.ClearProjectField(ctx, m.client, projectId)
				if err != nil {
					if errors.GetCause(err) == project.ErrEpochIsNotStable {
						continue
					}
					return errors.Tag(err, "cannot grant access")
				}
			}
		} else {
			request.Session.AddAnonTokenAccess(projectId, token)
		}
		response.RedirectTo = "/project/" + projectId.Hex()
		return nil
	}
	return project.ErrEpochIsNotStable
}
