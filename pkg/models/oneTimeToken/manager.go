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

package oneTimeToken

import (
	"context"
	"time"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	NewForEmailConfirmation(ctx context.Context, userId edgedb.UUID, email sharedTypes.Email) (OneTimeToken, error)
	NewForPasswordReset(ctx context.Context, userId edgedb.UUID, email sharedTypes.Email) (OneTimeToken, error)
	NewForPasswordSet(ctx context.Context, userId edgedb.UUID, email sharedTypes.Email) (OneTimeToken, error)
	ResolveAndExpireEmailConfirmationToken(ctx context.Context, token OneTimeToken) error
	ResolveAndExpirePasswordResetToken(ctx context.Context, token OneTimeToken, change *user.ForPasswordChange) error
}

func New(c *edgedb.Client) Manager {
	return &manager{c: c}
}

func rewriteEdgedbError(err error) error {
	if e, ok := err.(edgedb.Error); ok && e.Category(edgedb.NoDataError) {
		return &errors.NotFoundError{}
	}
	return err
}

const (
	emailConfirmationUse = "email_confirmation"
	passwordResetUse     = "password"
)

type manager struct {
	c *edgedb.Client
}

func (m *manager) NewForEmailConfirmation(ctx context.Context, userId edgedb.UUID, email sharedTypes.Email) (OneTimeToken, error) {
	return m.newToken(ctx, userId, email, emailConfirmationUse, time.Hour)
}

func (m *manager) NewForPasswordReset(ctx context.Context, userId edgedb.UUID, email sharedTypes.Email) (OneTimeToken, error) {
	return m.newForPasswordReset(ctx, userId, email, time.Hour)
}

func (m *manager) NewForPasswordSet(ctx context.Context, userId edgedb.UUID, email sharedTypes.Email) (OneTimeToken, error) {
	return m.newForPasswordReset(ctx, userId, email, 7*24*time.Hour)
}

func (m *manager) newForPasswordReset(ctx context.Context, userId edgedb.UUID, email sharedTypes.Email, validFor time.Duration) (OneTimeToken, error) {
	return m.newToken(ctx, userId, email, passwordResetUse, validFor)
}

func (m *manager) newToken(ctx context.Context, userId edgedb.UUID, email sharedTypes.Email, use string, expiresIn time.Duration) (OneTimeToken, error) {
	allErrors := &errors.MergedError{}
	for i := 0; i < 10; i++ {
		token, err := generateNewToken()
		if err != nil {
			allErrors.Add(err)
			continue
		}
		err = m.c.QuerySingle(ctx, `
insert OneTimeToken {
	expires_at := <datetime>$0,
	token := <str>$1,
	use := <str>$2,
	email := (
		select Email
		filter .email = <str>$3 and .user.id = <uuid>$4
	)
}
`,
			&IdField{},
			time.Now().UTC().Add(expiresIn), token, use, email, userId,
		)
		if err != nil {
			if e, ok := err.(edgedb.Error); ok {
				if e.Category(edgedb.ConstraintViolationError) {
					// Duplicate .token
					allErrors.Add(err)
					continue
				}
				if e.Category(edgedb.MissingRequiredError) {
					// Missing .email
					return "", &errors.UnprocessableEntityError{
						Msg: "account does not hold given email",
					}
				}
			}
			return "", rewriteEdgedbError(err)
		}
		return token, nil
	}
	return "", errors.Tag(allErrors, "bad random source")
}

func (m *manager) ResolveAndExpireEmailConfirmationToken(ctx context.Context, token OneTimeToken) error {
	err := m.c.QuerySingle(ctx, `
with
	t := (
		update OneTimeToken
		filter
				.use = <str>$0
			and .token = <str>$1
			and not exists .used_at
			and .expires_at > datetime_of_transaction()
		set {
			used_at := datetime_of_transaction()
		}
	)
update Email
filter Email = t.email
set { confirmed_at := datetime_of_transaction() }
`,
		&IdField{},
		emailConfirmationUse, token,
	)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return nil
}

func (m *manager) ResolveAndExpirePasswordResetToken(ctx context.Context, token OneTimeToken, u *user.ForPasswordChange) error {
	err := m.c.QuerySingle(ctx, `
with
	t := (
		update OneTimeToken
		filter
				.use = <str>$0
			and .token = <str>$1
			and not exists .used_at
			and .expires_at > datetime_of_transaction()
		set {
			used_at := datetime_of_transaction()
		}
	)
select User {
	email: { email },
	epoch,
	first_name,
	id,
	last_name,
	password_hash,
}
filter User = t.email.user
`,
		u,
		passwordResetUse, token,
	)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return nil
}
