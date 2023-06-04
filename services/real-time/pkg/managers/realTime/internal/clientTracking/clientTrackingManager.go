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
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime/internal/editorEvents"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

type Manager interface {
	Disconnect(client *types.Client) bool
	GetConnectedClients(ctx context.Context, client *types.Client) (types.ConnectedClients, error)
	Connect(ctx context.Context, client *types.Client, fetchConnectedUsers bool) types.ConnectedClients
	RefreshClientPositions(ctx context.Context, client map[sharedTypes.UUID]editorEvents.Clients) error
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
	ProjectExpiry    = 4 * 24 * time.Hour
	UserExpiry       = 15 * time.Minute
	RefreshUserEvery = UserExpiry - 1*time.Minute
)

func (m *manager) Disconnect(client *types.Client) bool {
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()

	n, err := m.deleteClientPosition(ctx, client)
	if err != nil || n > 0 {
		if errNotify := m.notifyDisconnected(ctx, client); errNotify != nil {
			err = errors.Merge(err, errNotify)
		}
	}
	if err != nil {
		err = errors.Tag(err, "disconnect connected client")
		log.Printf("%s/%s: %s", client.ProjectId, client.PublicId, err)
		return true
	}
	return n == 0
}

func (m *manager) Connect(ctx context.Context, client *types.Client, fetchConnectedUsers bool) types.ConnectedClients {
	clients, err := m.updateClientPosition(
		ctx, client, types.ClientPosition{}, fetchConnectedUsers,
	)
	if err != nil || len(clients) > 0 {
		if errNotify := m.notifyConnected(ctx, client); errNotify != nil {
			err = errors.Merge(errNotify, err)
		}
	}
	if err != nil {
		err = errors.Tag(err, "initialize connected client")
		log.Printf("%s/%s: %s", client.ProjectId, client.PublicId, err)
	}
	return clients
}

func (m *manager) UpdatePosition(ctx context.Context, client *types.Client, p types.ClientPosition) error {
	if err := m.notifyUpdated(ctx, client, p); err != nil {
		return err
	}
	if _, err := m.updateClientPosition(ctx, client, p, false); err != nil {
		return err
	}
	return nil
}
