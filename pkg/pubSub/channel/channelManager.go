// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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

	"github.com/redis/go-redis/v9"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type PubSubMessage struct {
	Msg     string
	Channel sharedTypes.UUID
}

type Message interface {
	ChannelId() sharedTypes.UUID
	json.Marshaler
}

type Writer interface {
	Publish(ctx context.Context, msg Message) error
	PublishVia(ctx context.Context, runner redis.Cmdable, msg Message) (*redis.IntCmd, error)
}

type Manager interface {
	Writer
	String() string
	Subscribe(ctx context.Context, id sharedTypes.UUID) error
	Unsubscribe(ctx context.Context, id sharedTypes.UUID)
	UnSubscribeBulk(ctx context.Context, ids sharedTypes.UUIDBatch) error
	Listen(ctx context.Context) (<-chan PubSubMessage, error)
	Close()
}

type BaseChannel string

type channel string

func (c BaseChannel) join(id sharedTypes.UUID) channel {
	b := make([]byte, 0, len(c)+1+36)
	b = append(b, c...)
	b = append(b, ':')
	b = id.Append(b)
	return channel(b)
}

func (c BaseChannel) parseIdFromChannel(s string) (sharedTypes.UUID, error) {
	if len(s) != len(c)+1+36 || s[len(c)] != ':' {
		return sharedTypes.UUID{}, errors.New("invalid channel format")
	}
	return sharedTypes.ParseUUID(s[len(c)+1:])
}

func newBatchGeneration() batchGeneration {
	return batchGeneration{
		queue: make(chan string, 1),
		done:  make(chan struct{}),
	}
}

type batchGeneration struct {
	queue      chan string
	done       chan struct{}
	err        error
	processing bool
}

func New(client redis.UniversalClient, baseChannel BaseChannel) Manager {
	m := manager{
		client:     client,
		base:       baseChannel,
		subQueue:   make(chan batchGeneration, 1),
		unSubQueue: make(chan batchGeneration, 1),
	}
	m.subQueue <- newBatchGeneration()
	m.unSubQueue <- newBatchGeneration()
	return &m
}

func NewWriter(client redis.UniversalClient, baseChannel BaseChannel) Writer {
	return New(client, baseChannel)
}

type manager struct {
	subQueue   chan batchGeneration
	unSubQueue chan batchGeneration
	client     redis.UniversalClient
	p          *redis.PubSub
	base       BaseChannel
	c          chan PubSubMessage
}

func (m *manager) String() string {
	return m.p.String()
}

func (m *manager) Subscribe(ctx context.Context, id sharedTypes.UUID) error {
	return m.batchSubscriptionChanges(ctx, id, m.subQueue, m.p.Subscribe)
}

func (m *manager) Unsubscribe(ctx context.Context, id sharedTypes.UUID) {
	// The pub/sub instance immediately "forgets" the channels that
	//  were unsubscribed from. When the operation fails, e.g. on
	//  connection errors, the pub/sub instance reconnects without the
	//  just "forgotten"  channels, hence we can ignore any errors.
	_ = m.batchSubscriptionChanges(ctx, id, m.unSubQueue, m.p.Unsubscribe)
	// We need to drop the room right away as we might never get a
	//  confirmation about the unsubscribe action -- e.g. when the
	//  connection errored.
	m.c <- PubSubMessage{Channel: id}
}

func (m *manager) UnSubscribeBulk(ctx context.Context, ids sharedTypes.UUIDBatch) error {
	n := ids.Len()
	args := make([]string, n)

	ids2 := ids
	for i := 0; i < n; i++ {
		args[i] = string(m.base.join(ids.Next()))
	}
	ids = ids2
	err := m.p.Unsubscribe(ctx, args...)
	for i := 0; i < n; i++ {
		m.c <- PubSubMessage{Channel: ids.Next()}
	}
	return err
}

func (m *manager) batchSubscriptionChanges(ctx context.Context, id sharedTypes.UUID, queue chan batchGeneration, fn func(ctx context.Context, channels ...string) error) error {
	bg := <-queue
	bg.queue <- string(m.base.join(id))
	if bg.processing == true {
		queue <- bg
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-bg.done:
			return bg.err
		}
	}
	bg.processing = true
	queue <- bg

	t := time.NewTimer(time.Millisecond)
	batch := []string{<-bg.queue}
waitForOthers:
	for {
		select {
		case other := <-bg.queue:
			batch = append(batch, other)
		case <-t.C:
			break waitForOthers
		}
	}
flush:
	for {
		select {
		case other, ok := <-bg.queue:
			if !ok {
				break flush
			}
			batch = append(batch, other)
		case <-queue:
			close(bg.queue)
		}
	}
	queue <- newBatchGeneration()
	if len(batch) > 1 {
		// Context cancellation should not abort the entire batch.
		ctx = context.Background()
	}
	ctx, done := context.WithTimeout(ctx, 10*time.Second)
	bg.err = fn(ctx, batch...)
	done()
	close(bg.done)
	return bg.err
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
	body, err := msg.MarshalJSON()
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

	m.c = make(chan PubSubMessage, 100)
	go m.listen(ctx)
	return m.c, nil
}

func (m *manager) listen(ctx context.Context) {
	defer close(m.c)
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
		if msg, ok := raw.(*redis.Message); ok {
			id, errId := m.base.parseIdFromChannel(msg.Channel)
			if errId != nil {
				continue
			}
			m.c <- PubSubMessage{
				Msg:     msg.Payload,
				Channel: id,
			}
		}
	}
}

func (m *manager) Close() {
	_ = m.p.Close()
}
