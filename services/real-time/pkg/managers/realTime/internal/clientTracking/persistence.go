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
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

const (
	userField     = "user"
	positionField = "cursorData"
)

func getConnectedUserKey(projectId sharedTypes.UUID, id sharedTypes.PublicId) string {
	return "connected_user:{" + projectId.String() + "}:" + string(id)
}

func getProjectKey(projectId sharedTypes.UUID) string {
	return "clients_in_project:{" + projectId.String() + "}"
}

func (m *manager) deleteClientPosition(ctx context.Context, client *types.Client) (bool, error) {
	projectKey := getProjectKey(client.ProjectId)
	userKey := getConnectedUserKey(client.ProjectId, client.PublicId)

	var remainingClients *redis.IntCmd
	_, err := m.redisClient.Pipelined(ctx, func(p redis.Pipeliner) error {
		p.SRem(ctx, projectKey, string(client.PublicId))
		remainingClients = p.SCard(ctx, projectKey)
		p.Del(ctx, userKey)
		return nil
	})
	if err != nil {
		return true, errors.Tag(err, "delete client position")
	}
	return remainingClients.Val() == 0, nil
}

func (m *manager) updateClientPosition(ctx context.Context, client *types.Client, position *types.ClientPosition) error {
	projectKey := getProjectKey(client.ProjectId)
	userKey := getConnectedUserKey(client.ProjectId, client.PublicId)

	userBlob, err := json.Marshal(client.User)
	if err != nil {
		return errors.Tag(err, "serialize user")
	}

	details := make([]interface{}, 0, 4)
	details = append(details, userField, string(userBlob))

	if position != nil {
		positionBlob, err2 := json.Marshal(position)
		if err2 != nil {
			return errors.Tag(err2, "serialize position")
		}
		details = append(details, positionField, positionBlob)
	}

	_, err = m.redisClient.TxPipelined(ctx, func(p redis.Pipeliner) error {
		p.SAdd(ctx, projectKey, string(client.PublicId))
		p.Expire(ctx, projectKey, ProjectExpiry)
		p.HSet(ctx, userKey, details...)
		p.Expire(ctx, userKey, UserExpiry)
		return nil
	})
	return err
}

func (m *manager) GetConnectedClients(ctx context.Context, client *types.Client) (types.ConnectedClients, error) {
	projectId := client.ProjectId
	rawIds, err := m.redisClient.SMembers(ctx, getProjectKey(projectId)).Result()
	if err != nil {
		return nil, err
	}
	if len(rawIds) == 0 ||
		len(rawIds) == 1 && rawIds[0] == string(client.PublicId) {
		// Fast path: no connected clients or just the RPC client.
		return make(types.ConnectedClients, 0), nil
	}
	ids := make([]sharedTypes.PublicId, len(rawIds))
	idxSelf := -1
	for idx, rawId := range rawIds {
		id := sharedTypes.PublicId(rawId)
		ids[idx] = id
		if id == client.PublicId {
			idxSelf = idx
		}
	}
	if idxSelf != -1 {
		if len(ids) > 1 {
			ids[idxSelf] = ids[len(ids)-1]
			ids = ids[:len(ids)-1]
		}
	}

	if err = ctx.Err(); err != nil {
		return nil, err
	}

	users := make([]*redis.StringStringMapCmd, len(ids))
	_, err = m.redisClient.Pipelined(ctx, func(p redis.Pipeliner) error {
		for idx, id := range ids {
			userKey := getConnectedUserKey(projectId, id)
			users[idx] = p.HGetAll(ctx, userKey)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	connectedClients := make(types.ConnectedClients, 0)
	staleClients := make([]sharedTypes.PublicId, 0)
	defer func() {
		if len(staleClients) != 0 {
			go m.cleanupStaleClientsInBackground(client.ProjectId, staleClients)
		}
	}()
	for idx, id := range ids {
		userDetails := users[idx].Val()
		userRaw := userDetails[userField]
		if userRaw == "" {
			staleClients = append(staleClients, id)
			continue
		}
		var user types.User
		err = json.Unmarshal([]byte(userRaw), &user)
		if err != nil {
			staleClients = append(staleClients, id)
			return nil, errors.Tag(err, "deserialize user: "+userRaw)
		}
		cc := types.ConnectedClient{
			ClientId: id,
			User:     user,
		}

		posRaw := userDetails[positionField]
		if posRaw != "" {
			var pos types.ClientPosition
			err = json.Unmarshal([]byte(posRaw), &pos)
			if err != nil {
				staleClients = append(staleClients, id)
				return nil, errors.Tag(
					err, "deserialize pos: "+posRaw,
				)
			}
			cc.ClientPosition = &pos
		}
		connectedClients = append(connectedClients, cc)
	}
	return connectedClients, nil
}

func (m *manager) RefreshClientPositions(ctx context.Context, clients []*types.Client, refreshProjectExpiry bool) error {
	if len(clients) == 0 {
		return nil
	}
	_, err := m.redisClient.Pipelined(ctx, func(p redis.Pipeliner) error {
		for idx, client := range clients {
			if idx == 0 && refreshProjectExpiry {
				projectKey := getProjectKey(client.ProjectId)
				p.Expire(ctx, projectKey, ProjectExpiry)
			}
			userKey := getConnectedUserKey(client.ProjectId, client.PublicId)
			p.Expire(ctx, userKey, UserExpiry)
		}
		return nil
	})
	return err
}

func (m *manager) cleanupStaleClientsInBackground(projectId sharedTypes.UUID, staleClients []sharedTypes.PublicId) {
	ctx, done := context.WithTimeout(context.Background(), 30*time.Second)
	defer done()

	if err := m.cleanupStaleClients(ctx, projectId, staleClients); err != nil {
		log.Printf(
			"%s: %s",
			projectId, errors.Tag(err, "error clearing stale clients"),
		)
	}
}

func (m *manager) cleanupStaleClients(ctx context.Context, projectId sharedTypes.UUID, staleClients []sharedTypes.PublicId) error {
	projectKey := getProjectKey(projectId)
	rawIds := make([]interface{}, len(staleClients))
	for idx, id := range staleClients {
		rawIds[idx] = string(id)
	}
	rawUserKeys := make([]string, len(staleClients))
	for idx, id := range staleClients {
		rawUserKeys[idx] = getConnectedUserKey(projectId, id)
	}
	_, err := m.redisClient.Pipelined(ctx, func(p redis.Pipeliner) error {
		p.Del(ctx, rawUserKeys...)
		p.SRem(ctx, projectKey, rawIds...)
		return nil
	})
	return err
}
