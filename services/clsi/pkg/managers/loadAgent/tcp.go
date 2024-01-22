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

package loadAgent

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

func NewServer(loadShedding bool, refreshCapacityEvery time.Duration) *Server {
	return &Server{
		getCapacity:  New(refreshCapacityEvery).GetCapacity,
		loadShedding: loadShedding,
	}
}

type Server struct {
	getCapacity  func() (int64, error)
	loadShedding bool
	closed       bool
	l            []net.Listener
	mu           sync.Mutex
}

func (s *Server) Shutdown(_ context.Context) error {
	s.mu.Lock()
	if !s.closed {
		for _, l := range s.l {
			_ = l.Close()
		}
		s.closed = true
	}
	s.mu.Unlock()
	return nil
}

func (s *Server) Serve(listener net.Listener) error {
	s.mu.Lock()
	if s.closed {
		return nil
	}
	s.l = append(s.l, listener)
	s.mu.Unlock()

	for {
		c, err := listener.Accept()
		if err != nil {
			if err == net.ErrClosed {
				return nil
			}
			// Backoff on error.
			time.Sleep(500 * time.Millisecond)
			continue
		}
		capacity, err := s.getCapacity()
		if err != nil {
			// Not sending a reply would count as a failed health check.
			// Emitting capacity=0 would trigger load shedding.
			// Only do that in case we are sure there is no capacity.
			capacity = 1
		}
		_ = c.SetWriteDeadline(time.Now().Add(time.Second))
		if s.loadShedding && capacity == 0 {
			_, _ = fmt.Fprintf(c, "maint, %d%%\n", capacity)
		} else {
			// 'ready' cancels out a previous 'maint' state.
			_, _ = fmt.Fprintf(c, "up, ready, %d%%\n", capacity)
		}
		_ = c.Close()
	}
}
