// Golang port of the Overleaf real-time service
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

package clientTracking

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/real-time/pkg/errors"
	"github.com/das7pad/real-time/pkg/types"
)

type Manager interface {
	DeleteClientPosition(client *types.Client)
	GetConnectedClients(ctx context.Context, client *types.Client) (types.ConnectedClients, error)
	InitializeClientPosition(client *types.Client)
	RefreshClientPositions(ctx context.Context, client []*types.Client, refreshProjectExpiry bool) error
	UpdateClientPosition(ctx context.Context, client *types.Client, position *types.ClientPosition) error
}

func New(client redis.UniversalClient) Manager {
	return &manager{redisClient: client}
}

type manager struct {
	redisClient redis.UniversalClient
}

const (
	ProjectExpiry       = 4 * 24 * time.Hour
	RefreshProjectEvery = ProjectExpiry - 12*time.Hour
	UserExpiry          = 15 * time.Minute
	RefreshUserEvery    = UserExpiry - 1*time.Minute

	userField     = "user"
	positionField = "cursorData"
)

func getConnectedUserKey(projectId primitive.ObjectID, id types.PublicId) string {
	return "connected_user:{" + projectId.Hex() + "}:" + string(id)
}

func getProjectKey(projectId primitive.ObjectID) string {
	return "clients_in_project:{" + projectId.Hex() + "}"
}

func (m *manager) GetConnectedClients(ctx context.Context, client *types.Client) (types.ConnectedClients, error) {
	projectId := *client.ProjectId
	ids, err := m.redisClient.SMembers(ctx, getProjectKey(projectId)).Result()
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 || len(ids) == 1 && ids[0] == string(client.PublicId) {
		return make(types.ConnectedClients, 0), err
	}
	for idx, id := range ids {
		if id == string(client.PublicId) {
			ids[idx] = ids[len(ids)-1]
			ids = ids[:len(ids)-1]
			break
		}
	}

	users := make([]*redis.StringStringMapCmd, len(ids))
	_, err = m.redisClient.Pipelined(ctx, func(p redis.Pipeliner) error {
		for idx, id := range ids {
			userKey := getConnectedUserKey(projectId, types.PublicId(id))
			users[idx] = p.HGetAll(ctx, userKey)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	clients := make(types.ConnectedClients, 0)
	for idx, id := range ids {
		userDetails := users[idx].Val()
		userRaw := userDetails[userField]
		if userRaw == "" {
			continue
		}
		var user types.User
		err = json.Unmarshal([]byte(userRaw), &user)
		if err != nil {
			return nil, errors.Tag(err, "cannot deserialize user: "+userRaw)
		}
		client := &types.ConnectedClient{
			ClientId: types.PublicId(id),
			User:     user,
		}

		posRaw := userDetails[positionField]
		if posRaw != "" {
			var pos types.ClientPosition
			err = json.Unmarshal([]byte(posRaw), &pos)
			if err != nil {
				return nil, errors.Tag(
					err, "cannot deserialize pos: "+posRaw,
				)
			}
			client.ClientPosition = &pos
		}
		clients = append(clients, client)
	}
	return clients, nil
}

func (m *manager) DeleteClientPosition(client *types.Client) {
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()

	projectKey := getProjectKey(*client.ProjectId)
	userKey := getConnectedUserKey(*client.ProjectId, client.PublicId)
	_, err := m.redisClient.Pipelined(ctx, func(p redis.Pipeliner) error {
		p.SRem(ctx, projectKey, string(client.PublicId))
		p.Expire(ctx, projectKey, ProjectExpiry)
		p.Del(ctx, userKey)
		return nil
	})
	if err != nil {
		log.Println("error deleting client position: " + err.Error())
	}
}

func (m *manager) InitializeClientPosition(client *types.Client) {
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()

	_ = m.UpdateClientPosition(ctx, client, nil)
}

func (m *manager) RefreshClientPositions(ctx context.Context, clients []*types.Client, refreshProjectExpiry bool) error {
	if len(clients) == 0 {
		return nil
	}
	_, err := m.redisClient.Pipelined(ctx, func(p redis.Pipeliner) error {
		for idx, client := range clients {
			if idx == 0 && refreshProjectExpiry {
				projectKey := getProjectKey(*client.ProjectId)
				p.Expire(ctx, projectKey, ProjectExpiry)
			}
			userKey := getConnectedUserKey(*client.ProjectId, client.PublicId)
			p.Expire(ctx, userKey, UserExpiry)
		}
		return nil
	})
	return err
}

func (m *manager) UpdateClientPosition(ctx context.Context, client *types.Client, position *types.ClientPosition) error {
	projectKey := getProjectKey(*client.ProjectId)
	userKey := getConnectedUserKey(*client.ProjectId, client.PublicId)

	user, err := json.Marshal(client.User)
	if err != nil {
		return errors.Tag(err, "cannot serialize user")
	}

	details := make(map[string]interface{})
	details[userField] = string(user)

	if position != nil {
		pos, err2 := json.Marshal(position)
		if err2 != nil {
			return errors.Tag(err2, "cannot serialize position")
		}
		details[positionField] = pos
	}

	_, err = m.redisClient.Pipelined(ctx, func(p redis.Pipeliner) error {
		p.SAdd(ctx, projectKey, string(client.PublicId))
		p.Expire(ctx, projectKey, ProjectExpiry)
		p.HSet(ctx, userKey, details)
		p.Expire(ctx, userKey, UserExpiry)
		return nil
	})
	return err
}
