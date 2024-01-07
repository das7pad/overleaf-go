// Golang port of Overleaf
// Copyright (C) 2023-2024 Jakob Ackermann <das7pad@outlook.com>
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
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime/internal/editorEvents"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

func getProjectKey(projectId sharedTypes.UUID) string {
	b := make([]byte, 0, 16+36+1)
	b = append(b, "clientTracking:{"...)
	b = projectId.Append(b)
	b = append(b, '}')
	return string(b)
}

func (m *manager) updateClientPosition(ctx context.Context, client *types.Client, position types.ClientPosition) error {
	userBlob, err := json.Marshal(types.ConnectedClient{
		ClientPosition: position,
		DisplayName:    client.DisplayName,
	})
	if err != nil {
		return errors.Tag(err, "serialize connected user")
	}

	projectKey := getProjectKey(client.ProjectId)
	details := []interface{}{
		string(client.PublicId),
		userBlob,
		string(client.PublicId) + ":age",
		strconv.FormatInt(time.Now().Unix(), 36),
	}
	_, err = m.redisClient.TxPipelined(ctx, func(p redis.Pipeliner) error {
		p.HSet(ctx, projectKey, details...)
		p.Expire(ctx, projectKey, ProjectExpiry)
		return nil
	})
	if err != nil {
		return errors.Tag(err, "persist client details")
	}
	return nil
}

func (m *manager) GetConnectedClients(ctx context.Context, client *types.Client) (json.RawMessage, error) {
	pending, ownsPending := m.pcc[client.ProjectId[0]].get(client.ProjectId)
	if ownsPending {
		time.Sleep(time.Millisecond)
		projectKey := getProjectKey(client.ProjectId)
		entries, err := m.redisClient.HGetAll(ctx, projectKey).Result()
		if err != nil {
			pending.err = err
		} else {
			pending.clients, pending.err =
				m.parseConnectedClients(client.ProjectId, entries)
		}
		m.pcc[client.ProjectId[0]].delete(client.ProjectId)
		close(pending.done)
	} else {
		<-pending.done
	}
	return pending.clients, pending.err
}

func (m *manager) parseConnectedClients(projectId sharedTypes.UUID, entries map[string]string) (json.RawMessage, error) {
	var staleClients []sharedTypes.PublicId
	defer func() {
		if len(staleClients) == 0 {
			return
		}
		go m.cleanupStaleClientsInBackground(projectId, staleClients)
	}()

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
	blob, err := json.Marshal(clients)
	if err != nil {
		return nil, errors.Tag(err, "serialize clients")
	}
	return blob, nil
}

func (m *manager) RefreshClientPositions(ctx context.Context, rooms map[sharedTypes.UUID]editorEvents.Clients) error {
	merged := errors.MergedError{}
	for len(rooms) > 0 {
		_, err := m.redisClient.TxPipelined(ctx, func(p redis.Pipeliner) error {
			now := strconv.FormatInt(time.Now().Unix(), 36)
			n := 0
			for projectId, clients := range rooms {
				fields := make([]interface{}, 2*len(clients.All))
				for i, client := range clients.All {
					fields[2*i] = string(client.PublicId) + ":age"
					fields[2*i+1] = now
				}
				if clients.Removed != -1 {
					fields[2*clients.Removed] = fields[len(fields)-2]
					fields[2*clients.Removed+1] = fields[len(fields)-1]
					fields = fields[:len(fields)-2]
				}
				projectKey := getProjectKey(projectId)
				p.HSet(ctx, projectKey, fields...)
				p.Expire(ctx, projectKey, ProjectExpiry)
				delete(rooms, projectId)
				n += len(clients.All)
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
