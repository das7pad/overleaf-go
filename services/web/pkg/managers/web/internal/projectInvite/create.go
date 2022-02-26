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

package projectInvite

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/projectInvite"
	"github.com/das7pad/overleaf-go/pkg/mongoTx"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func generateNewToken() (projectInvite.Token, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", errors.Tag(err, "cannot generate new token")
	}
	return projectInvite.Token(hex.EncodeToString(b)), nil
}

func (m *manager) CreateProjectInvite(ctx context.Context, request *types.CreateProjectInviteRequest) error {
	request.Preprocess()
	if err := request.Validate(); err != nil {
		return err
	}

	token, err := generateNewToken()
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	pi := &projectInvite.WithToken{}
	pi.CreatedAt = now
	pi.Email = request.Email
	pi.Expires = now.Add(30 * 24 * time.Hour)
	// TODO: refactor into server side gen.
	pi.Id = edgedb.UUID{}
	pi.PrivilegeLevel = request.PrivilegeLevel
	pi.ProjectId = request.ProjectId
	pi.SendingUserId = request.SenderUserId
	pi.Token = token

	d, err := m.getDetails(ctx, pi)
	if err != nil {
		return err
	}
	if err = d.ValidateForCreation(); err != nil {
		return err
	}

	err = mongoTx.For(m.db, ctx, func(ctx context.Context) error {
		if err2 := m.pim.Create(ctx, pi); err2 != nil {
			return errors.Tag(err2, "cannot create invite")
		}
		if err2 := m.createNotification(ctx, d); err2 != nil {
			return errors.Tag(err2, "cannot create notification")
		}
		return nil
	})

	// Possible false negative error, request refresh.
	defer m.notifyEditorAboutChanges(
		request.ProjectId, &refreshMembershipDetails{Invites: true},
	)

	if err != nil {
		return errors.Tag(err, "cannot persist invitation")
	}

	if err = m.sendEmail(ctx, d); err != nil {
		return err
	}
	return nil
}
