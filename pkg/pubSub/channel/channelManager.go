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

package channel

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type PubSubMessage struct {
	Msg     string
	Channel sharedTypes.UUID
}

type Message interface {
	ChannelId() sharedTypes.UUID
}

type Writer interface {
	Publish(ctx context.Context, msg Message) error
	PublishVia(ctx context.Context, runner redis.Cmdable, msg Message) (*redis.IntCmd, error)
}

type Manager interface {
	Writer
	Subscribe(ctx context.Context, id sharedTypes.UUID) error
	Unsubscribe(ctx context.Context, id sharedTypes.UUID)
	Listen(ctx context.Context) (<-chan PubSubMessage, error)
	Close()
}

type BaseChannel string

type channel string

func (c BaseChannel) join(id sharedTypes.UUID) channel {
	return channel(string(c) + ":" + id.String())
}

func (c BaseChannel) parseIdFromChannel(s string) (sharedTypes.UUID, error) {
	if len(s) != len(c)+36+1 {
		return sharedTypes.UUID{}, errors.New("invalid channel format")
	}
	return sharedTypes.ParseUUID(s[len(c)+1:])
}

func New(client redis.UniversalClient, baseChannel BaseChannel) Manager {
	return &manager{
		client: client,
		base:   baseChannel,
	}
}

func NewWriter(client redis.UniversalClient, baseChannel BaseChannel) Writer {
	return New(client, baseChannel)
}

type manager struct {
	client redis.UniversalClient
	p      *redis.PubSub
	base   BaseChannel
	c      chan PubSubMessage
}

func (m *manager) Subscribe(ctx context.Context, id sharedTypes.UUID) error {
	return m.p.Subscribe(ctx, string(m.base.join(id)))
}

func (m *manager) Unsubscribe(ctx context.Context, id sharedTypes.UUID) {
	// The pub/sub instance immediately "forgets" the channels that
	//  were unsubscribed from. When the operation fails, e.g. on
	//  connection errors, the pub/sub instance reconnects without the
	//  just "forgotten"  channels, hence we can ignore any errors.
	_ = m.p.Unsubscribe(ctx, string(m.base.join(id)))
	// We need to drop the room right away as we might never get a
	//  confirmation about the unsubscribe action -- e.g. when the
	//  connection errored.
	m.c <- PubSubMessage{Channel: id}
}

func (m *manager) Publish(ctx context.Context, msg Message) error {
	cmd, err := m.PublishVia(ctx, m.client, msg)
	if err != nil {
		return err
	}
	if err = cmd.Err(); err != nil {
		return errors.Tag(err, "publish message")
	}
	return nil
}

func (m *manager) PublishVia(ctx context.Context, runner redis.Cmdable, msg Message) (*redis.IntCmd, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, errors.Tag(err, "encode message for publishing")
	}
	id := msg.ChannelId()
	return runner.Publish(ctx, string(m.base.join(id)), body), nil
}

func (m *manager) Listen(ctx context.Context) (<-chan PubSubMessage, error) {
	m.p = m.client.Subscribe(ctx, string(m.base.join(sharedTypes.UUID{})))
	if _, err := m.p.Receive(ctx); err != nil {
		return nil, err
	}

	rawC := make(chan PubSubMessage, 100)
	m.c = rawC
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
				log.Printf(
					"pubsub receive: nFailed=%d, %q", nFailed, err.Error(),
				)
				time.Sleep(time.Duration(math.Min(
					float64(5*time.Second),
					math.Pow(2, float64(nFailed))*float64(time.Millisecond),
				)))
				continue
			}
			nFailed = 0
			switch msg := raw.(type) {
			case *redis.Message:
				id, errId := m.base.parseIdFromChannel(msg.Channel)
				if errId != nil {
					continue
				}
				rawC <- PubSubMessage{
					Msg:     msg.Payload,
					Channel: id,
				}
			}
		}
	}()
	return rawC, nil
}

func (m *manager) Close() {
	_ = m.p.Close()
}
