// Golang port of the Overleaf clsi service
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

package main

import (
	"io"
	"net"
	"strconv"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi"
)

func startLoadAgent(options *clsiOptions, manager clsi.Manager) (io.Closer, error) {
	l, listenErr := net.Listen("tcp", options.loadAddress)
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
			capacity, err := manager.GetCapacity()
			if err != nil {
				// 0 would instruct haproxy to stop sending traffic.
				// Only do that in case we are sure there is no capacity.
				capacity = 1
			}
			msg := "up, " + strconv.FormatInt(capacity, 10) + "%\n"
			_, _ = c.Write([]byte(msg))
			_ = c.Close()
		}
	}()
	return listener, nil
}
