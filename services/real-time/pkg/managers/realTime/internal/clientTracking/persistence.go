// Golang port of Overleaf
// Copyright (C) 2023 Jakob Ackermann <das7pad@outlook.com>
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
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

func getProjectKey(projectId sharedTypes.UUID) string {
	return "clientTracking:{" + projectId.String() + "}"
}

func (m *manager) deleteClientPosition(ctx context.Context, client *types.Client) (int64, error) {
	projectKey := getProjectKey(client.ProjectId)

	var remainingClients *redis.IntCmd
	_, err := m.redisClient.Pipelined(ctx, func(p redis.Pipeliner) error {
		f := string(client.PublicId)
		p.HDel(ctx, projectKey, f, f+":age")
		remainingClients = p.Exists(ctx, projectKey)
		return nil
	})
	if err != nil {
		return -1, errors.Tag(err, "delete client position")
	}
	return remainingClients.Val(), nil
}

func (m *manager) updateClientPosition(ctx context.Context, client *types.Client, position *types.ClientPosition, fetchConnectedUsers bool) (types.ConnectedClients, error) {
	projectKey := getProjectKey(client.ProjectId)

	userBlob, err := json.Marshal(types.ConnectedClient{
		User:           client.User,
		ClientPosition: position,
	})
	if err != nil {
		return nil, errors.Tag(err, "serialize connected user")
	}

	details := []interface{}{
		string(client.PublicId),
		string(userBlob),
		string(client.PublicId) + ":age",
		strconv.FormatInt(time.Now().Unix(), 36),
	}

	var existingClients *redis.StringStringMapCmd
	_, err = m.redisClient.TxPipelined(ctx, func(p redis.Pipeliner) error {
		if fetchConnectedUsers {
			existingClients = p.HGetAll(ctx, projectKey)
		}
		p.HSet(ctx, projectKey, details...)
		p.Expire(ctx, projectKey, ProjectExpiry)
		return nil
	})
	if err != nil {
		return nil, errors.Tag(err, "persist client details")
	}
	if fetchConnectedUsers {
		return m.parseConnectedClients(client, existingClients)
	}
	return nil, nil
}

func (m *manager) GetConnectedClients(ctx context.Context, client *types.Client) (types.ConnectedClients, error) {
	return m.parseConnectedClients(
		client,
		m.redisClient.HGetAll(ctx, getProjectKey(client.ProjectId)),
	)
}

func (m *manager) parseConnectedClients(client *types.Client, cmd *redis.StringStringMapCmd) (types.ConnectedClients, error) {
	if err := cmd.Err(); err != nil {
		return nil, errors.Tag(err, "get raw connected clients")
	}

	var staleClients []sharedTypes.PublicId
	defer func() {
		if len(staleClients) == 0 {
			return
		}
		go m.cleanupStaleClientsInBackground(client.ProjectId, staleClients)
	}()

	entries := cmd.Val()
	// omit self
	delete(entries, string(client.PublicId))
	delete(entries, string(client.PublicId)+":age")

	tStale := time.Now().Add(-UserExpiry).Unix()
	for k, v := range entries {
		if clientId, _, isAge := strings.Cut(k, ":age"); isAge {
			delete(entries, k)
			if t, _ := strconv.ParseInt(v, 36, 64); t < tStale {
				staleClients = append(
					staleClients, sharedTypes.PublicId(clientId),
				)
				delete(entries, clientId)
			}
		}
	}
	clients := make(types.ConnectedClients, len(entries))
	i := 0
	for clientId, v := range entries {
		if err := json.Unmarshal([]byte(v), &clients[i]); err != nil {
			return nil, errors.Tag(err, "deserialize client")
		}
		clients[i].ClientId = sharedTypes.PublicId(clientId)
		i++
	}
	return clients, nil
}

func (m *manager) RefreshClientPositions(ctx context.Context, rooms map[sharedTypes.UUID][]*types.Client) error {
	merged := errors.MergedError{}
	for len(rooms) > 0 {
		_, err := m.redisClient.TxPipelined(ctx, func(p redis.Pipeliner) error {
			n := 0
			for projectId, clients := range rooms {
				fields := make([]interface{}, 2*len(clients))
				now := strconv.FormatInt(time.Now().Unix(), 36)
				for i, client := range clients {
					fields[2*i] = string(client.PublicId) + ":age"
					fields[2*i+1] = now
				}
				projectKey := getProjectKey(projectId)
				p.HSet(ctx, projectKey, fields...)
				p.Expire(ctx, projectKey, ProjectExpiry)
				delete(rooms, projectId)
				n += len(clients)
				if n >= 100 {
					break
				}
			}
			return nil
		})
		merged.Add(err)
	}
	return merged.Finalize()
}

func (m *manager) cleanupStaleClientsInBackground(projectId sharedTypes.UUID, staleClients []sharedTypes.PublicId) {
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()

	key := getProjectKey(projectId)
	fields := make([]string, 2*len(staleClients))
	for idx, id := range staleClients {
		fields[2*idx] = string(id)
		fields[2*idx+1] = string(id) + ":age"
	}
	if err := m.redisClient.HDel(ctx, key, fields...).Err(); err != nil {
		log.Printf(
			"%s: %s",
			projectId, errors.Tag(err, "error clearing stale clients"),
		)
	}
}