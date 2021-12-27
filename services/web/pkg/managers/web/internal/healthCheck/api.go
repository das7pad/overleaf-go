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

package healthCheck

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

func (m *manager) smokeTestRedis(ctx context.Context) error {
	rawRand := make([]byte, 4)
	if _, err := rand.Read(rawRand); err != nil {
		return errors.Tag(err, "cannot get random blob")
	}
	perRequestRnd := hex.EncodeToString(rawRand)
	key := fmt.Sprintf("%s:%s", m.randomPrefix, perRequestRnd)
	err := m.client.SetEX(ctx, key, perRequestRnd, 10*time.Second).Err()
	if err != nil {
		return errors.Tag(err, "cannot write")
	}
	res, err := m.client.Get(ctx, key).Result()
	if err != nil {
		return errors.Tag(err, "cannot read")
	}
	if res != perRequestRnd {
		return &failure{Msg: "read mismatch"}
	}
	return nil
}

func (m *manager) smokeTestMongo(ctx context.Context) error {
	if _, err := m.um.GetEpoch(ctx, m.smokeTestUserId); err != nil {
		return err
	}
	return nil
}

func (m *manager) SmokeTestAPI(ctx context.Context) error {
	ctx, done := context.WithTimeout(ctx, 5*time.Second)
	defer done()
	if err := m.smokeTestRedis(ctx); err != nil {
		return errors.Tag(err, "redis check failed")
	}
	if err := m.smokeTestMongo(ctx); err != nil {
		return errors.Tag(err, "mongo check failed")
	}
	return nil
}
