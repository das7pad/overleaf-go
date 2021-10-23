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

package channel

import (
	"context"
	"math"
	"time"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Action int

const (
	Message Action = iota
	Unsubscribed
)

type PubSubMessage struct {
	Msg     string
	Channel primitive.ObjectID
	Action  Action
}

type Manager interface {
	Subscribe(ctx context.Context, id primitive.ObjectID) error
	Unsubscribe(ctx context.Context, id primitive.ObjectID) error
	Publish(ctx context.Context, id primitive.ObjectID, msg string) error
	Listen(ctx context.Context) (<-chan *PubSubMessage, error)
	Close()
}

type BaseChannel string
type channel string

func (c BaseChannel) join(id primitive.ObjectID) channel {
	return channel(string(c) + ":" + id.Hex())
}

func (c BaseChannel) parseIdFromChannel(s string) primitive.ObjectID {
	if len(s) != len(c)+25 {
		return primitive.NilObjectID
	}
	id, err := primitive.ObjectIDFromHex(s[len(c)+1:])
	if err != nil {
		return primitive.NilObjectID
	}
	return id
}

func New(client redis.UniversalClient, baseChannel BaseChannel) Manager {
	return &manager{
		client: client,
		base:   baseChannel,
	}
}

type manager struct {
	client redis.UniversalClient
	p      *redis.PubSub
	base   BaseChannel
}

func (m *manager) Subscribe(ctx context.Context, id primitive.ObjectID) error {
	return m.p.Subscribe(ctx, string(m.base.join(id)))
}

func (m *manager) Unsubscribe(ctx context.Context, id primitive.ObjectID) error {
	return m.p.Unsubscribe(ctx, string(m.base.join(id)))
}

func (m *manager) Publish(ctx context.Context, id primitive.ObjectID, msg string) error {
	return m.client.Publish(ctx, string(m.base.join(id)), msg).Err()
}

func (m *manager) Listen(ctx context.Context) (<-chan *PubSubMessage, error) {
	m.p = m.client.Subscribe(ctx, string(m.base))
	if _, err := m.p.Receive(ctx); err != nil {
		return nil, err
	}

	rawC := make(chan *PubSubMessage, 100)
	go func() {
		defer close(rawC)
		nFailed := 0
		for {
			raw, err := m.p.Receive(ctx)
			if err != nil {
				if err == redis.ErrClosed {
					return
				}
				nFailed++
				time.Sleep(time.Duration(math.Min(
					float64(5*time.Second),
					math.Pow(2, float64(nFailed))*float64(time.Millisecond),
				)))
				continue
			}
			nFailed = 0
			switch msg := raw.(type) {
			case *redis.Subscription:
				if msg.Kind != "unsubscribe" {
					continue
				}
				rawC <- &PubSubMessage{
					Channel: m.base.parseIdFromChannel(msg.Channel),
					Action:  Unsubscribed,
				}
			case *redis.Message:
				rawC <- &PubSubMessage{
					Msg:     msg.Payload,
					Channel: m.base.parseIdFromChannel(msg.Channel),
					Action:  Message,
				}
			}
		}
	}()
	return rawC, nil
}

func (m *manager) Close() {
	_ = m.p.Close()
}
