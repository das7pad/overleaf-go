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

package session

import (
	"encoding/json"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/oneTimeToken"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type sessionValidationToken string

type sessionDataWithDeSyncProtection struct {
	ValidationToken sessionValidationToken `json:"!"`
	Data
}

type sessionDataWithDeSyncProtectionOld struct {
	ValidationToken    sessionValidationToken    `json:"validationToken"`
	AnonTokenAccess    anonTokenAccess           `json:"anonTokenAccess,omitempty"`
	PasswordResetToken oneTimeToken.OneTimeToken `json:"resetToken,omitempty"`
	PostLoginRedirect  string                    `json:"postLoginRedirect,omitempty"`
	User               *struct {
		Email          sharedTypes.Email `json:"email"`
		FirstName      string            `json:"first_name,omitempty"`
		IPAddress      string            `json:"ip_address"`
		Id             sharedTypes.UUID  `json:"_id,omitempty"`
		LastName       string            `json:"last_name,omitempty"`
		Language       string            `json:"lng"`
		SessionCreated time.Time         `json:"session_created"`
	} `json:"user,omitempty"`
}

var ErrRedisConnectionDeSync = errors.New("redis connection de-synced")

func (s sessionValidationToken) Validate(id Id) error {
	if s != id.toSessionValidationToken() {
		return ErrRedisConnectionDeSync
	}
	return nil
}

func serializeSession(id Id, data Data) ([]byte, error) {
	return json.Marshal(sessionDataWithDeSyncProtection{
		ValidationToken: id.toSessionValidationToken(),
		Data:            data,
	})
}

func deSerializeSession(id Id, blob []byte) (*Data, error) {
	var dst sessionDataWithDeSyncProtection
	if err := json.Unmarshal(blob, &dst); err != nil {
		return deSerializeOldSession(id, blob)
	}
	if err := dst.ValidationToken.Validate(id); err != nil {
		return deSerializeOldSession(id, blob)
	}
	return &dst.Data, nil
}

func deSerializeOldSession(id Id, blob []byte) (*Data, error) {
	var dst sessionDataWithDeSyncProtectionOld
	if err := json.Unmarshal(blob, &dst); err != nil {
		return nil, err
	}
	if err := dst.ValidationToken.Validate(id); err != nil {
		return nil, err
	}
	var l string
	var lm *LoginMetadata
	var u *User
	if dst.User != nil {
		l = dst.User.Language
		lm = &LoginMetadata{
			IPAddress:  dst.User.IPAddress,
			LoggedInAt: dst.User.SessionCreated,
		}
		u = &User{
			Email:     dst.User.Email,
			FirstName: dst.User.FirstName,
			Id:        dst.User.Id,
			LastName:  dst.User.LastName,
		}
	}
	return &Data{
		AnonTokenAccess:    dst.AnonTokenAccess,
		PasswordResetToken: dst.PasswordResetToken,
		PostLoginRedirect:  dst.PostLoginRedirect,
		LoginMetadata:      lm,
		PublicData: PublicData{
			Language: l,
			User:     u,
		},
		isOldSchema: true,
	}, nil
}
