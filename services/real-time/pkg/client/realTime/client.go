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

package realTime

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

type Client struct {
	conn           *websocket.Conn
	mu             sync.Mutex
	nextCB         types.Callback
	callbacks      map[types.Callback]func(response types.RPCResponse)
	listener       map[string]func(response types.RPCResponse)
	stopPingTicker func()
}

var closeMessage *websocket.PreparedMessage

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopPingTicker != nil {
		c.stopPingTicker()
		c.stopPingTicker = nil
	}
	if c.conn != nil {
		_ = c.conn.WritePreparedMessage(closeMessage)
		_ = c.conn.Close()
		c.conn = nil
	}
}

var nextId = atomic.Int64{}

func (c *Client) Connect(ctx context.Context, url, bootstrap string) (*types.RPCResponse, error) {
	id := nextId.Add(1)
	conn, _, err := (&websocket.Dialer{
		HandshakeTimeout: time.Minute,
		Subprotocols: []string{
			"v8.real-time.overleaf.com",
			bootstrap + ".bootstrap.v8.real-time.overleaf.com",
		},
	}).DialContext(ctx, url+"?id="+strconv.FormatInt(id, 10), nil)
	if err != nil {
		return nil, fmt.Errorf("%d: dial: %w", id, err)
	}
	c.conn = conn
	c.callbacks = make(map[types.Callback]func(response types.RPCResponse))
	c.listener = make(map[string]func(response types.RPCResponse))

	res := types.RPCResponse{}
	c.On("bootstrap", func(response types.RPCResponse) {
		res = response
	})
	c.On("connectionRejected", func(response types.RPCResponse) {
		res = response
	})
	c.On("clientTracking.clientConnected", func(_ types.RPCResponse) {
	})
	c.On("clientTracking.clientDisconnected", func(_ types.RPCResponse) {
	})

	d := time.Now().Add(time.Minute)
	if err = c.conn.SetReadDeadline(d); err != nil {
		c.Close()
		return nil, fmt.Errorf("%d: set deadline: %w", id, err)
	}
	for res.Name == "" {
		if err = c.ReadOnce(); err != nil {
			c.Close()
			return nil, fmt.Errorf("%d: readOnce: %w", id, err)
		}
	}
	if res.Name == "connectionRejected" {
		c.Close()
		err = errors.New("connectionRejected")
		body := string(res.Body)
		return &res, fmt.Errorf("%d: body=%s: %w", id, body, err)
	}
	return &res, nil
}

func (c *Client) Ping() error {
	return c.RPC(&types.RPCResponse{}, &types.RPCRequest{Action: "ping"})
}

func (c *Client) StartHealthCheck() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return errors.New("closed")
	}
	var t *time.Timer
	t = time.AfterFunc(time.Second*30, func() {
		if err := c.Ping(); err != nil {
			log.Printf("health check: %s", err)
			c.Close()
		} else {
			t.Reset(time.Second * 30)
		}
	})
	c.stopPingTicker = func() {
		t.Stop()
	}
	return nil
}

func (c *Client) On(name string, fn func(response types.RPCResponse)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.listener[name] = fn
}

func (c *Client) RPCAsyncWrite(res *types.RPCResponse, r *types.RPCRequest) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return errors.New("closed")
	}

	d := time.Now().Add(10 * time.Second)
	if err := c.conn.SetWriteDeadline(d); err != nil {
		return err
	}
	if err := c.conn.SetReadDeadline(d); err != nil {
		return err
	}
	c.nextCB++
	r.Callback = c.nextCB
	c.callbacks[r.Callback] = func(response types.RPCResponse) {
		*res = response
	}
	if err := c.conn.WriteJSON(r); err != nil {
		return err
	}
	return nil
}

func (c *Client) RPCAsyncRead(r *types.RPCRequest) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for c.callbacks[r.Callback] != nil {
		if err := c.ReadOnce(); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) RPC(res *types.RPCResponse, r *types.RPCRequest) error {
	if err := c.RPCAsyncWrite(res, r); err != nil {
		return err
	}
	return c.RPCAsyncRead(r)
}

func (c *Client) ReadOnce() error {
	if c.conn == nil {
		return errors.New("closed")
	}
	res := types.RPCResponse{}
	if err := c.conn.ReadJSON(&res); err != nil {
		return err
	}
	matched := false
	if res.Callback != 0 {
		matched = true
		c.callbacks[res.Callback](res)
		delete(c.callbacks, res.Callback)
	} else if res.Name != "" {
		if l := c.listener[res.Name]; l != nil {
			matched = true
			l(res)
		}
	}
	for _, lr := range res.LazySuccessResponses {
		c.callbacks[lr.Callback](types.RPCResponse{
			Latency: lr.Latency,
		})
		delete(c.callbacks, lr.Callback)
	}
	if !matched {
		body := string(res.Body)
		return fmt.Errorf("unmatched read: body=%s res=%#v", body, res)
	}
	return nil
}

func init() {
	data := websocket.FormatCloseMessage(websocket.CloseGoingAway, "")
	var err error
	closeMessage, err = websocket.NewPreparedMessage(
		websocket.CloseMessage, data,
	)
	if err != nil {
		panic(err)
	}
}
