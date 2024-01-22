// Golang port of Overleaf
// Copyright (C) 2024 Jakob Ackermann <das7pad@outlook.com>
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

package httpUtils

import (
	"context"
	"net"
	"os"
	"strings"
)

type Server interface {
	Serve(listener net.Listener) error
	Shutdown(ctx context.Context) error
}

func ListenAndServe(server Server, addr string) error {
	var l net.Listener
	var err error
	if strings.HasPrefix(addr, "/") {
		if err = os.Remove(addr); err != nil && !os.IsNotExist(err) {
			return err
		}
		l, err = net.Listen("unix", addr)
	} else {
		l, err = net.Listen("tcp", addr)
	}
	if err != nil {
		return err
	}
	return server.Serve(l)
}

func ListenAndServeEach(do func(func() error), server Server, each []string) {
	for _, s := range each {
		addr := s
		do(func() error {
			return ListenAndServe(server, addr)
		})
	}
}
