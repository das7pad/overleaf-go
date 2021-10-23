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

package systemMessage

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/pkg/models/systemMessage"
)

type Manager interface {
	systemMessage.Manager
	GetAllCached(ctx context.Context, userId primitive.ObjectID) []systemMessage.Full
}

type manager struct {
	systemMessage.Manager

	l       sync.Mutex
	expires time.Time
	cached  []systemMessage.Full
}

var noMessages = make([]systemMessage.Full, 0)

func New(db *mongo.Database) Manager {
	return &manager{
		Manager: systemMessage.New(db),
		cached:  noMessages,
	}
}

func (m *manager) GetAllCached(ctx context.Context, userId primitive.ObjectID) []systemMessage.Full {
	if userId.IsZero() {
		// Hide messages for logged out users.
		return noMessages
	}
	if m.expires.After(time.Now()) {
		// Happy path
		return m.cached
	}
	m.l.Lock()
	defer m.l.Unlock()
	if m.expires.After(time.Now()) {
		// Another goroutine refreshed the cache already.
		return m.cached
	}
	messages, err := m.Manager.GetAll(ctx)
	if err != nil {
		// Ignore refresh errors.
		return m.cached
	}
	m.cached = messages
	jitter := time.Duration(rand.Int63n(int64(2 * time.Second)))
	m.expires = time.Now().Add(10*time.Second + jitter)
	return messages
}