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

package clientTracking

import (
	"context"
	"log"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/pubSub/channel"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

type Manager interface {
	Disconnect(client *types.Client) bool
	GetConnectedClients(ctx context.Context, client *types.Client) (types.ConnectedClients, error)
	ConnectInBackground(client *types.Client)
	RefreshClientPositions(ctx context.Context, client []*types.Client, refreshProjectExpiry bool) error
	UpdatePosition(ctx context.Context, client *types.Client, position types.ClientPosition) error
}

func New(client redis.UniversalClient, c channel.Writer) Manager {
	return &manager{
		redisClient: client,
		c:           c,
	}
}

type manager struct {
	redisClient redis.UniversalClient
	c           channel.Writer
}

const (
	ProjectExpiry       = 4 * 24 * time.Hour
	RefreshProjectEvery = ProjectExpiry - 12*time.Hour
	UserExpiry          = 15 * time.Minute
	RefreshUserEvery    = UserExpiry - 1*time.Minute
)

func (m *manager) Disconnect(client *types.Client) bool {
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()

	nowEmpty, errUpdate := m.deleteClientPosition(ctx, client)
	errNotify := m.notifyDisconnected(ctx, client)
	if err := errors.Merge(errNotify, errUpdate); err != nil {
		err = errors.Tag(err, "disconnect connected client")
		log.Printf("%s/%s: %s", client.ProjectId, client.PublicId, err)
		return true
	}
	return nowEmpty
}

func (m *manager) ConnectInBackground(client *types.Client) {
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()

	errInitUser := m.updateClientPosition(ctx, client, nil)
	errNotify := m.notifyConnected(ctx, client)
	if err := errors.Merge(errNotify, errInitUser); err != nil {
		err = errors.Tag(err, "initialize connected client")
		log.Printf("%s/%s: %s", client.ProjectId, client.PublicId, err)
	}
}

func (m *manager) UpdatePosition(ctx context.Context, client *types.Client, p types.ClientPosition) error {
	if err := m.notifyUpdated(ctx, client, p); err != nil {
		return err
	}
	if err := m.updateClientPosition(ctx, client, &p); err != nil {
		return err
	}
	return nil
}
