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

package channel

import (
	"context"

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
	Listen(ctx context.Context) <-chan *PubSubMessage
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

func New(ctx context.Context, client redis.UniversalClient, baseChannel BaseChannel) (Manager, error) {
	p := client.Subscribe(ctx, string(baseChannel))

	if _, err := p.Receive(ctx); err != nil {
		return nil, err
	}

	return &manager{
		client: client,
		p:      p,
		base:   baseChannel,
	}, nil
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

func (m *manager) Listen(ctx context.Context) <-chan *PubSubMessage {
	rawC := make(chan *PubSubMessage, 100)
	go func() {
		defer close(rawC)
		for {
			raw, err := m.p.Receive(ctx)
			if err != nil {
				if err == redis.ErrClosed {
					return
				}
				continue
			}
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
	return rawC
}

func (m *manager) Close() {
	_ = m.p.Close()
}
