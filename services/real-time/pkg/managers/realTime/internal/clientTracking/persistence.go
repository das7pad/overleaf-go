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
	"encoding/binary"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/das7pad/overleaf-go/pkg/base64Ordered"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime/internal/editorEvents"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

func encodeAge(t time.Time) [11]byte {
	buf := [11]byte{}
	binary.BigEndian.PutUint64(buf[3:], uint64(t.UnixNano()))
	base64Ordered.Encode(buf[:], buf[3:])
	return buf
}

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
	now := encodeAge(time.Now())
	details := []interface{}{
		string(client.PublicId),
		userBlob,
		string(client.PublicId) + ":age",
		string(now[:]),
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
			pending.clients = m.buildConnectedClients(client.ProjectId, entries)
		}
		close(pending.done)
		m.pcc[client.ProjectId[0]].delete(client.ProjectId)
	} else {
		<-pending.done
	}
	return pending.clients, pending.err
}

var noClients = []byte("[]")

func (m *manager) buildConnectedClients(projectId sharedTypes.UUID, entries map[string]string) json.RawMessage {
	var staleClients []sharedTypes.PublicId
	defer func() {
		if len(staleClients) == 0 {
			return
		}
		go m.cleanupStaleClientsInBackground(projectId, staleClients)
	}()

	tStale := encodeAge(time.Now().Add(-UserExpiry))
	n := 0
	for k, v := range entries {
		if isAge := strings.HasSuffix(k, ":age"); isAge {
			if v < string(tStale[:]) {
				clientId := strings.TrimSuffix(k, ":age")
				staleClients = append(
					staleClients, sharedTypes.PublicId(clientId),
				)
				delete(entries, clientId)
			}
			continue
		}
		n += 6 + len(k) + 2 + len(v) - 1 + 1
	}
	if n == 0 {
		return noClients
	}
	blob := make([]byte, 0, 1+n-1+1)
	blob = append(blob, '[')
	for k, v := range entries {
		if isAge := strings.HasSuffix(k, ":age"); isAge || len(v) == 0 {
			continue
		}
		blob = append(blob, `{"i":"`...)
		blob = append(blob, k...)
		blob = append(blob, `",`...)
		blob = append(blob, v[1:]...)
		blob = append(blob, ',')
	}
	blob[len(blob)-1] = ']'
	return blob
}

func (m *manager) RefreshClientPositions(ctx context.Context, rooms editorEvents.LazyRoomClients) error {
	merged := errors.MergedError{}
	args := make([]interface{}, 0, 2*100)
	now := make([]byte, 11)
	for len(rooms) > 0 {
		_, err := m.redisClient.TxPipelined(ctx, func(p redis.Pipeliner) error {
			b := encodeAge(time.Now())
			copy(now, b[:])
			n := 0
			for projectId, r := range rooms {
				clients := r.Clients()
				if c := 2 * len(clients.All); cap(args) < c {
					args = make([]interface{}, 0, c)
				}
				args = args[:0]
				for i, client := range clients.All {
					if clients.Removed.Has(i) {
						continue
					}
					args = append(args, string(client.PublicId)+":age", now)
				}
				clients.Done()
				projectKey := getProjectKey(projectId)
				p.HSet(ctx, projectKey, args...)
				p.Expire(ctx, projectKey, ProjectExpiry)
				delete(rooms, projectId)
				n += len(args)
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
	key := getProjectKey(projectId)
	fields := make([]string, 2*len(staleClients))
	for idx, id := range staleClients {
		fields[2*idx] = string(id)
		fields[2*idx+1] = string(id) + ":age"
	}
	ctx := context.Background() // rely on connection timeout
	if err := m.redisClient.HDel(ctx, key, fields...).Err(); err != nil {
		log.Printf(
			"%s: %s",
			projectId, errors.Tag(err, "error clearing stale clients"),
		)
	}
}
