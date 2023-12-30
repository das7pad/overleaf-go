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

package cache

import (
	"sync"
)

func NewLimited[K comparable, V any](size int) *Limited[K, V] {
	return &Limited[K, V]{
		items: make(map[K]V, size),
		size:  size,
	}
}

type Limited[K comparable, V any] struct {
	mu    sync.RWMutex
	items map[K]V
	size  int
}

func (c *Limited[K, V]) Get(k K) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.items[k]
	return v, ok
}

func (c *Limited[K, V]) Add(k K, v V) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.items[k]; exists {
		c.items[k] = v
		return false
	}
	evict := len(c.items) >= c.size
	if evict {
		c.deleteOnePseudoRandomItem()
	}
	c.items[k] = v
	return evict
}

func (c *Limited[K, V]) deleteOnePseudoRandomItem() {
	for k := range c.items {
		delete(c.items, k)
		break
	}
}
