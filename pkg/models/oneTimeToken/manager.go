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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	NewForEmailConfirmation(ctx context.Context, userId primitive.ObjectID, email sharedTypes.Email) (OneTimeToken, error)
	NewForPasswordReset(ctx context.Context, userId primitive.ObjectID, email sharedTypes.Email) (OneTimeToken, error)
	ResolveAndExpireEmailConfirmationToken(ctx context.Context, token OneTimeToken) (*EmailConfirmationData, error)
	ResolveAndExpirePasswordResetToken(ctx context.Context, token OneTimeToken) (*PasswordResetData, error)
}

func New(db *mongo.Database) Manager {
	return &manager{
		c: db.Collection("tokens"),
	}
}

func rewriteMongoError(err error) error {
	if err == mongo.ErrNoDocuments {
		return &errors.NotFoundError{}
	}
	return err
}

const (
	emailConfirmationUse = "email_confirmation"
	passwordResetUse     = "password"
)

type manager struct {
	c *mongo.Collection
}

func (m *manager) NewForEmailConfirmation(ctx context.Context, userId primitive.ObjectID, email sharedTypes.Email) (OneTimeToken, error) {
	now := time.Now().UTC()
	return m.newToken(ctx, func(token OneTimeToken) interface{} {
		return &forEmailConfirmation{
			UseField: UseField{
				Use: emailConfirmationUse,
			},
			TokenField: TokenField{
				Token: token,
			},
			EmailConfirmationDataField: EmailConfirmationDataField{
				EmailConfirmationData: EmailConfirmationData{
					Email:  email,
					UserId: userId,
				},
			},
			CreatedAtField: CreatedAtField{
				CreatedAt: now,
			},
			ExpiresAtField: ExpiresAtField{
				ExpiresAt: now.Add(time.Hour),
			},
		}
	})
}

func (m *manager) NewForPasswordReset(ctx context.Context, userId primitive.ObjectID, email sharedTypes.Email) (OneTimeToken, error) {
	now := time.Now().UTC()
	return m.newToken(ctx, func(token OneTimeToken) interface{} {
		return &forNewPasswordReset{
			UseField: UseField{
				Use: passwordResetUse,
			},
			TokenField: TokenField{
				Token: token,
			},
			PasswordResetDataField: PasswordResetDataField{
				PasswordResetData: PasswordResetData{
					Email:     email,
					HexUserId: userId.Hex(),
				},
			},
			CreatedAtField: CreatedAtField{
				CreatedAt: now,
			},
			ExpiresAtField: ExpiresAtField{
				ExpiresAt: now.Add(time.Hour),
			},
		}
	})
}

func (m *manager) newToken(ctx context.Context, fn func(token OneTimeToken) interface{}) (OneTimeToken, error) {
	allErrors := &errors.MergedError{}
	for i := 0; i < 10; i++ {
		token, err := generateNewToken()
		if err != nil {
			allErrors.Add(err)
			continue
		}
		_, err = m.c.InsertOne(ctx, fn(token))
		if err != nil {
			if mongo.IsDuplicateKeyError(err) {
				allErrors.Add(err)
				continue
			}
			return "", rewriteMongoError(err)
		}
		return token, nil
	}
	return "", errors.Tag(allErrors, "bad random source")
}

func (m *manager) ResolveAndExpireEmailConfirmationToken(ctx context.Context, token OneTimeToken) (*EmailConfirmationData, error) {
	res := &EmailConfirmationDataField{}
	err := m.resolveAndExpireToken(ctx, emailConfirmationUse, token, res)
	if err != nil {
		return nil, rewriteMongoError(err)
	}
	return &res.EmailConfirmationData, nil
}

func (m *manager) ResolveAndExpirePasswordResetToken(ctx context.Context, token OneTimeToken) (*PasswordResetData, error) {
	res := &PasswordResetDataField{}
	err := m.resolveAndExpireToken(ctx, passwordResetUse, token, res)
	if err != nil {
		return nil, rewriteMongoError(err)
	}
	return &res.PasswordResetData, nil
}

func (m *manager) resolveAndExpireToken(ctx context.Context, use string, token OneTimeToken, target interface{}) error {
	now := time.Now().UTC()
	q := bson.M{
		"use":   use,
		"token": token,
		"expiresAt": bson.M{
			"$gt": now,
		},
		"usedAt": bson.M{
			"$exists": false,
		},
	}
	u := bson.M{
		"$set": UsedAtField{
			UsedAt: now,
		},
	}
	err := m.c.FindOneAndUpdate(
		ctx, q, u,
		options.FindOneAndUpdate().SetProjection(getProjection(target)),
	).Decode(target)
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}
