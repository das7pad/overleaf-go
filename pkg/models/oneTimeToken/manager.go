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
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4/pgxpool"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	NewForEmailConfirmation(ctx context.Context, userId sharedTypes.UUID, email sharedTypes.Email) (OneTimeToken, error)
	NewForPasswordReset(ctx context.Context, userId sharedTypes.UUID, email sharedTypes.Email) (OneTimeToken, error)
	ResolveAndExpireEmailConfirmationToken(ctx context.Context, token OneTimeToken) error
}

func New(db *pgxpool.Pool) Manager {
	return &manager{db: db}
}

const (
	EmailConfirmationUse = "email_confirmation"
	PasswordResetUse     = "password"
)

type manager struct {
	db *pgxpool.Pool
}

func (m *manager) NewForEmailConfirmation(ctx context.Context, userId sharedTypes.UUID, email sharedTypes.Email) (OneTimeToken, error) {
	return m.newToken(ctx, userId, email, EmailConfirmationUse, time.Hour)
}

func (m *manager) NewForPasswordReset(ctx context.Context, userId sharedTypes.UUID, email sharedTypes.Email) (OneTimeToken, error) {
	return m.newToken(ctx, userId, email, PasswordResetUse, time.Hour)
}

func (m *manager) newToken(ctx context.Context, userId sharedTypes.UUID, email sharedTypes.Email, use string, expiresIn time.Duration) (OneTimeToken, error) {
	allErrors := &errors.MergedError{}
	for i := 0; i < 10; i++ {
		token, err := GenerateNewToken()
		if err != nil {
			allErrors.Add(err)
			continue
		}
		r, err := m.db.Exec(ctx, `
INSERT INTO one_time_tokens
(created_at, email, expires_at, token, use, user_id)
SELECT transaction_timestamp(), $2, $3, $4, $5, u.id
FROM users u
WHERE u.id = $1
  AND u.email = $2
  AND u.deleted_at IS NULL
`, userId, email, time.Now().Add(expiresIn), token, use)
		if err != nil {
			if e, ok := err.(*pgconn.PgError); ok &&
				e.ConstraintName == "one_time_tokens_pkey" {
				// Duplicate .token
				allErrors.Add(err)
				continue
			}
			return "", err
		}
		if r.RowsAffected() == 0 {
			return "", &errors.UnprocessableEntityError{
				Msg: "account does not hold given email",
			}
		}
		return token, nil
	}
	return "", errors.Tag(allErrors, "bad random source")
}

func (m *manager) ResolveAndExpireEmailConfirmationToken(ctx context.Context, token OneTimeToken) error {
	r, err := m.db.Exec(ctx, `
WITH ott AS (
    UPDATE one_time_tokens
        SET used_at = transaction_timestamp()
        WHERE token = $1 AND use = $2 AND used_at IS NULL
        RETURNING email)
UPDATE users u
SET email_confirmed_at = transaction_timestamp()
FROM ott
WHERE u.email = ott.email
  AND u.deleted_at IS NULL
  AND u.email_confirmed_at IS NULL
`, token, EmailConfirmationUse)
	if err != nil {
		return err
	}
	if r.RowsAffected() == 0 {
		return &errors.NotFoundError{}
	}
	return nil
}
