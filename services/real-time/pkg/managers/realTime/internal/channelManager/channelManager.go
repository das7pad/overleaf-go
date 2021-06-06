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

package channelManager

import (
	"context"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Manager interface {
	Subscribe(ctx context.Context, id primitive.ObjectID) error
	Unsubscribe(ctx context.Context, id primitive.ObjectID) error
	Publish(ctx context.Context, id primitive.ObjectID, msg string) error
	Listen() <-chan string
	Close()
}

type BaseChannel string
type channel string

func (c BaseChannel) join(id primitive.ObjectID) channel {
	return channel(string(c) + ":" + id.Hex())
}

func New(ctx context.Context, c redis.UniversalClient, baseChannel BaseChannel) (Manager, error) {
	p := c.Subscribe(ctx, string(baseChannel))

	if _, err := p.Receive(ctx); err != nil {
		return nil, err
	}

	return &manager{
		c: c,
		p: p,
	}, nil
}

type manager struct {
	c    redis.UniversalClient
	p    *redis.PubSub
	base BaseChannel
}

func (m *manager) Subscribe(ctx context.Context, id primitive.ObjectID) error {
	return m.p.Subscribe(ctx, string(m.base.join(id)))
}

func (m *manager) Unsubscribe(ctx context.Context, id primitive.ObjectID) error {
	return m.p.Unsubscribe(ctx, string(m.base.join(id)))
}

func (m *manager) Publish(ctx context.Context, id primitive.ObjectID, msg string) error {
	return m.c.Publish(ctx, string(m.base.join(id)), msg).Err()
}

func (m *manager) Listen() <-chan string {
	rawC := make(chan string)
	go func() {
		for msg := range m.p.Channel() {
			rawC <- msg.Payload
		}
		close(rawC)
	}()
	return rawC
}

func (m *manager) Close() {
	_ = m.p.Close()
}
