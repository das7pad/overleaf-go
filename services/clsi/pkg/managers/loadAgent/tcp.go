// Golang port of Overleaf
// Copyright (C) 2021-2022 Jakob Ackermann <das7pad@outlook.com>
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
	"fmt"
	"io"
	"net"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

func Start(addr string, loadShedding bool, refreshCapacityEvery time.Duration) (io.Closer, error) {
	return startLoadAgent(addr, loadShedding, New(refreshCapacityEvery).GetCapacity)
}

func startLoadAgent(addr string, loadShedding bool, getCapacity func() (int64, error)) (io.Closer, error) {
	l, listenErr := net.Listen("tcp", addr)
	if listenErr != nil {
		return nil, listenErr
	}

	listener, ok := l.(*net.TCPListener)
	if !ok {
		return nil, errors.New("listening for tcp should yield TCPListener")
	}

	go func() {
		for {
			c, err := listener.AcceptTCP()
			if err != nil {
				if err == net.ErrClosed {
					break
				}
				// Backoff on error.
				time.Sleep(500 * time.Millisecond)
				continue
			}
			capacity, err := getCapacity()
			if err != nil {
				// Not sending a reply would count as a failed health check.
				// Emitting capacity=0 would trigger load shedding.
				// Only do that in case we are sure there is no capacity.
				capacity = 1
			}
			var msg string
			if loadShedding && capacity == 0 {
				msg = fmt.Sprintf("maint, %d%%\n", capacity)
			} else {
				// 'ready' cancels out a previous 'maint' state.
				msg = fmt.Sprintf("up, ready, %d%%\n", capacity)
			}
			_, _ = c.Write([]byte(msg))
			_ = c.Close()
		}
	}()
	return listener, nil
}
