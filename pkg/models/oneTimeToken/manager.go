// Golang port of Overleaf
// Copyright (C) 2021-2022 Jakob Ackermann <das7pad@outlook.com>
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
	"database/sql"
	"time"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	NewForEmailConfirmation(ctx context.Context, userId sharedTypes.UUID, email sharedTypes.Email) (OneTimeToken, error)
	NewForPasswordReset(ctx context.Context, userId sharedTypes.UUID, email sharedTypes.Email) (OneTimeToken, error)
	NewForPasswordSet(ctx context.Context, userId sharedTypes.UUID, email sharedTypes.Email) (OneTimeToken, error)
	ResolveAndExpireEmailConfirmationToken(ctx context.Context, token OneTimeToken) error
}

func New(db *sql.DB) Manager {
	return &manager{db: db}
}

func rewriteEdgedbError(err error) error {
	if e, ok := err.(edgedb.Error); ok && e.Category(edgedb.NoDataError) {
		return &errors.NotFoundError{}
	}
	return err
}

const (
	EmailConfirmationUse = "email_confirmation"
	PasswordResetUse     = "password"
)

type manager struct {
	c  *edgedb.Client
	db *sql.DB
}

func (m *manager) NewForEmailConfirmation(ctx context.Context, userId sharedTypes.UUID, email sharedTypes.Email) (OneTimeToken, error) {
	return m.newToken(ctx, userId, email, EmailConfirmationUse, time.Hour)
}

func (m *manager) NewForPasswordReset(ctx context.Context, userId sharedTypes.UUID, email sharedTypes.Email) (OneTimeToken, error) {
	return m.newForPasswordReset(ctx, userId, email, time.Hour)
}

func (m *manager) NewForPasswordSet(ctx context.Context, userId sharedTypes.UUID, email sharedTypes.Email) (OneTimeToken, error) {
	return m.newForPasswordReset(ctx, userId, email, 7*24*time.Hour)
}

func (m *manager) newForPasswordReset(ctx context.Context, userId sharedTypes.UUID, email sharedTypes.Email, validFor time.Duration) (OneTimeToken, error) {
	return m.newToken(ctx, userId, email, PasswordResetUse, validFor)
}

func (m *manager) newToken(ctx context.Context, userId sharedTypes.UUID, email sharedTypes.Email, use string, expiresIn time.Duration) (OneTimeToken, error) {
	allErrors := &errors.MergedError{}
	for i := 0; i < 10; i++ {
		token, err := GenerateNewToken()
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
		filter
			.email = <str>$3
		and .user.id = <uuid>$4
		and not exists .user.deleted_at
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
			and not exists .email.user.deleted_at
		set {
			used_at := datetime_of_transaction()
		}
	)
update t.email
set { confirmed_at := datetime_of_transaction() }
`,
		&IdField{},
		EmailConfirmationUse, token,
	)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return nil
}
