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

package healthCheck

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/pendingOperation"
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

func (m *manager) smokeTestEdgedb(ctx context.Context) error {
	u := &user.BetaProgramField{}
	if err := m.um.GetUser(ctx, m.smokeTestUserId, u); err != nil {
		return err
	}
	return nil
}

func (m *manager) smokeTestAPI() error {
	ctx, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()
	eg, pCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		if err := m.smokeTestRedis(pCtx); err != nil {
			return errors.Tag(err, "redis check failed")
		}
		return nil
	})
	eg.Go(func() error {
		if err := m.smokeTestEdgedb(pCtx); err != nil {
			return errors.Tag(err, "edgedb check failed")
		}
		return nil
	})
	return eg.Wait()
}

func (m *manager) getPendingOrStartApiSmokeTest() pendingOperation.PendingOperation {
	m.apiMux.Lock()
	defer m.apiMux.Unlock()
	if p := m.apiPending; p != nil && m.apiValidUntil.After(time.Now()) {
		return p
	}
	p := pendingOperation.TrackOperation(func() error {
		return m.smokeTestAPI()
	})
	m.apiPending = p
	m.apiValidUntil = time.Now().Add(5 * time.Second)
	return p
}

func (m *manager) SmokeTestAPI(ctx context.Context) error {
	return m.getPendingOrStartApiSmokeTest().Wait(ctx)
}
