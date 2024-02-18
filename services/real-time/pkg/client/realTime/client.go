// Golang port of Overleaf
// Copyright (C) 2023-2024 Jakob Ackermann <das7pad@outlook.com>
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
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/randQueue"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

type Client struct {
	conn           *websocket.Conn
	mu             sync.Mutex
	nextCB         types.Callback
	callbacks      map[types.Callback]func(response types.RPCResponse)
	listener       []listener
	stopPingTicker func()
	buf            *rwBuffer
}

type listener struct {
	name sharedTypes.EditorEventMessage
	fn   func(response types.RPCResponse)
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
	if c.buf != nil {
		putBuffer(c.buf)
		c.buf = nil
	}
}

var localhostAddr = &net.TCPAddr{
	IP:   net.IPv4(127, 0, 0, 1),
	Port: 3026,
}

var UnixRunRealTime = &net.UnixAddr{
	Net:  "unix",
	Name: "/tmp/real-time.socket",
}

type ConnectFn = func(ctx context.Context, network, addr string) (net.Conn, error)

func DialLocalhost(_ context.Context, _, _ string) (net.Conn, error) {
	return net.DialTCP("tcp4", nil, localhostAddr)
}

func DialUnix(_ context.Context, _, _ string) (net.Conn, error) {
	return net.DialUnix("unix", nil, UnixRunRealTime)
}

var nextId = atomic.Int64{}

var (
	separatorColon        = []byte(":")
	separatorComma        = []byte(",")
	headerKeyConnection   = []byte("Connection")
	headerValueConnection = []byte("Upgrade")
	headerKeyUpgrade      = []byte("Upgrade")
	headerValueUpgrade    = []byte("websocket")
	headerKeyWSProtocol   = []byte("Sec-Websocket-Protocol")
	headerValueWSProtocol = []byte("v8.real-time.overleaf.com")
	headerKeyWSAccept     = []byte("Sec-Websocket-Accept")
	responseLine          = []byte("HTTP/1.1 101 Switching Protocols\r\n")
)

var rng = make(randQueue.Q16, 512)

func init() {
	go rng.Run(4096)
}

func (c *Client) connect(ctx context.Context, uri *url.URL, bootstrap string, dial ConnectFn) error {
	w, err := dial(ctx, "", "")
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer func() {
		if c.conn == nil {
			_ = w.Close()
		}
	}()

	if _, ok := ctx.Deadline(); ok || ctx.Done() != nil {
		cancelCtx, done := context.WithCancel(ctx)
		abortDone := make(chan struct{})
		defer func() {
			<-abortDone
		}()
		defer done()
		go func() {
			<-cancelCtx.Done()
			if c.conn == nil {
				_ = w.Close()
			}
			close(abortDone)
		}()
	}

	c.buf = newBuffer(w)
	key := c.buf.WriteBuffer[0:24]
	p := c.buf.WriteBuffer[24:24]

	rnd := <-rng
	base64.StdEncoding.Encode(key, rnd[:])

	p = append(p, "GET "...)
	p = append(p, uri.Path...)
	p = append(p, " HTTP/1.1\r\n"...)
	p = append(p, "Host: "...)
	p = append(p, uri.Host...)
	p = append(p, "\r\n"...)
	p = append(p, "Connection: Upgrade\r\n"...)
	p = append(p, "Upgrade: websocket\r\n"...)
	p = append(p, "Sec-Websocket-Version: 13\r\n"...)
	p = append(p, "Sec-Websocket-Protocol: v8.real-time.overleaf.com, "...)
	p = append(p, bootstrap...)
	p = append(p, ".bootstrap.v8.real-time.overleaf.com\r\n"...)
	p = append(p, "Sec-Websocket-Key: "...)
	p = append(p, key...)
	p = append(p, "\r\n\r\n"...)

	if _, err = w.Write(p); err != nil {
		return fmt.Errorf("write request: %w", err)
	}

	l, err := c.buf.ReadSlice('\n')
	if err != nil {
		return fmt.Errorf("read response line: %w", err)
	}
	if !bytes.Equal(l, responseLine) {
		return fmt.Errorf("response line: %s", string(l))
	}

	accept := appendSecWebSocketAccept(p[:0], key)
	var checks [4]bool

	for {
		l, err = c.buf.ReadSlice('\n')
		if err != nil {
			return fmt.Errorf("read header line: %w", err)
		}
		if len(l) <= 2 {
			break
		}

		name, value, ok := bytes.Cut(l, separatorColon)
		value = bytes.TrimSpace(value)
		if !ok || len(name) == 0 || len(value) == 0 {
			return fmt.Errorf("header line: %s", string(l))
		}
		switch {
		case !checks[0] && bytes.EqualFold(name, headerKeyConnection):
			var next []byte
			for ok && len(value) > 0 {
				next, value, ok = bytes.Cut(value, separatorComma)
				value = bytes.TrimSpace(value)
				if bytes.EqualFold(next, headerValueConnection) {
					checks[0] = true
					break
				}
			}
		case !checks[1] && bytes.EqualFold(name, headerKeyUpgrade):
			checks[1] = bytes.EqualFold(value, headerValueUpgrade)
		case !checks[2] && bytes.EqualFold(name, headerKeyWSProtocol):
			checks[2] = bytes.Equal(value, headerValueWSProtocol)
		case !checks[3] && bytes.EqualFold(name, headerKeyWSAccept):
			checks[3] = bytes.Equal(value, accept)
			if !checks[3] {
				return fmt.Errorf(
					"accept want=%s got=%s", string(accept), string(value),
				)
			}
		}
	}

	for i, ok := range checks {
		if !ok {
			return fmt.Errorf("check failed %d", i)
		}
	}

	c.conn = websocket.NewConn(w, false, 2048, 2048, nil, c.buf.Reader, c.buf.WriteBuffer)
	return nil
}

func (c *Client) Connect(ctx context.Context, uri *url.URL, bootstrap string, dial ConnectFn) (*types.RPCResponse, error) {
	id := nextId.Add(1)
	if err := c.connect(ctx, uri, bootstrap, dial); err != nil {
		return nil, fmt.Errorf("%d: %w", id, err)
	}
	c.callbacks = make(map[types.Callback]func(response types.RPCResponse))
	c.listener = make([]listener, 0, 5)

	res := types.RPCResponse{}
	c.On(sharedTypes.Bootstrap, func(response types.RPCResponse) {
		res = response
	})
	c.On(sharedTypes.ConnectionRejected, func(response types.RPCResponse) {
		res = response
	})
	c.On(sharedTypes.ClientTrackingBatch, func(_ types.RPCResponse) {
	})
	c.On(sharedTypes.ClientTrackingUpdated, func(_ types.RPCResponse) {
	})

	if deadline, ok := ctx.Deadline(); ok {
		if err := c.conn.SetReadDeadline(deadline); err != nil {
			c.Close()
			return nil, fmt.Errorf("%d: set deadline: %w", id, err)
		}
	}
	for res.Name == "" {
		if err := c.ReadOnce(); err != nil {
			c.Close()
			return nil, fmt.Errorf("%d: readOnce: %w", id, err)
		}
	}
	if res.Name == sharedTypes.ConnectionRejected {
		c.Close()
		err := errors.New(string(res.Name))
		body := string(res.Body)
		return &res, fmt.Errorf("%d: body=%s: %w", id, body, err)
	}
	return &res, nil
}

func (c *Client) Ping() error {
	return c.RPC(&types.RPCResponse{}, &types.RPCRequest{Action: types.Ping})
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

func (c *Client) On(name sharedTypes.EditorEventMessage, fn func(response types.RPCResponse)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.listener = append(c.listener, listener{
		name: name,
		fn:   fn,
	})
}

func (c *Client) SetDeadline(d time.Time) error {
	if err := c.conn.SetWriteDeadline(d); err != nil {
		return err
	}
	if err := c.conn.SetReadDeadline(d); err != nil {
		return err
	}
	return nil
}

func (c *Client) RPCAsyncWrite(res *types.RPCResponse, r *types.RPCRequest) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return errors.New("closed")
	}

	c.nextCB++
	if len(c.callbacks) == 0 {
		c.nextCB = 1
	}
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
	_, r, err := c.conn.NextReader()
	if err != nil {
		return err
	}
	c.buf.ReadBuffer.Reset()
	if _, err = c.buf.ReadBuffer.ReadFrom(r); err != nil {
		return err
	}
	res := types.RPCResponse{}
	if err = res.FastUnmarshalJSON(c.buf.ReadBuffer.Bytes()); err != nil {
		return err
	}
	matched := false
	if res.Callback != 0 {
		matched = true
		c.callbacks[res.Callback](res)
		delete(c.callbacks, res.Callback)
	} else if res.Name != "" {
		for _, l := range c.listener {
			if l.name == res.Name {
				l.fn(res)
				matched = true
			}
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

var wsKeyGUID = []byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11")

func appendSecWebSocketAccept(buf []byte, k []byte) []byte {
	h := sha1.New()
	h.Write(k)
	h.Write(wsKeyGUID)
	k = h.Sum(k[:0])
	return base64.StdEncoding.AppendEncode(buf, k)
}

var bufferPool sync.Pool

type rwBuffer struct {
	*bufio.Reader
	WriteBuffer []byte
	ReadBuffer  *bytes.Buffer
}

func newBuffer(r io.Reader) *rwBuffer {
	if v := bufferPool.Get(); v != nil {
		br := v.(*rwBuffer)
		br.Reader.Reset(r)
		return br
	}
	return &rwBuffer{
		Reader:      bufio.NewReaderSize(r, 2048),
		WriteBuffer: make([]byte, 2048),
		ReadBuffer:  bytes.NewBuffer(make([]byte, 0, 4096)),
	}
}

func putBuffer(br *rwBuffer) {
	br.Reader.Reset(nil)
	bufferPool.Put(br)
	return
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
