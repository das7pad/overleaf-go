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
	conn           websocket.LeanConn
	mu             sync.Mutex
	nextCB         types.Callback
	callbacks      map[types.Callback]func(response types.RPCResponse)
	listenerFixed  [4]listener
	listenerExtra  []listener
	stopPingTicker func()
	buf            *readBuffer
}

type listener struct {
	name sharedTypes.EditorEventMessage
	fn   func(response types.RPCResponse)
}

var closeMessage *websocket.PreparedMessage

func (c *Client) AnnounceClose() error {
	return c.conn.WritePreparedMessage(closeMessage)
}

func (c *Client) CloseWrite() error {
	cr := c.conn.Conn.(interface{ CloseWrite() error })
	return cr.CloseWrite()
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopPingTicker != nil {
		c.stopPingTicker()
		c.stopPingTicker = nil
	}
	if c.conn.Conn != nil {
		_ = c.conn.Close()
		c.conn.Conn = nil
	}
	if c.buf != nil {
		putReadBuffer(c.buf)
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

func equalFoldASCII(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	return bytes.EqualFold(a, b)
}

func (c *Client) connect(ctx context.Context, uri *url.URL, bootstrap string, dial ConnectFn) error {
	w, err := dial(ctx, "", "")
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer func() {
		if c.conn.Conn == nil {
			_ = w.Close()
		}
	}()

	if ctx.Done() != nil {
		cancelCtx, done := context.WithCancel(ctx)
		abortDone := make(chan struct{})
		defer func() {
			<-abortDone
		}()
		defer done()
		go func() {
			<-cancelCtx.Done()
			c.mu.Lock()
			if c.conn.Conn == nil {
				_ = w.Close()
			}
			c.mu.Unlock()
			close(abortDone)
		}()
	}

	c.buf = newReadBuffer(w)
	wBuf := writeBufferPool.Get().(*writeBuf)
	defer writeBufferPool.Put(wBuf)

	key := wBuf.p[0:24]
	p := wBuf.p[24:24]

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
	wBuf.p = p

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
		case !checks[0] && equalFoldASCII(name, headerKeyConnection):
			var next []byte
			for ok && len(value) > 0 {
				next, value, ok = bytes.Cut(value, separatorComma)
				value = bytes.TrimSpace(value)
				if equalFoldASCII(next, headerValueConnection) {
					checks[0] = true
					break
				}
			}
		case !checks[1] && equalFoldASCII(name, headerKeyUpgrade):
			checks[1] = equalFoldASCII(value, headerValueUpgrade)
		case !checks[2] && equalFoldASCII(name, headerKeyWSProtocol):
			checks[2] = bytes.Equal(value, headerValueWSProtocol)
		case !checks[3] && equalFoldASCII(name, headerKeyWSAccept):
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

	c.mu.Lock()
	c.conn = websocket.LeanConn{
		Conn:                        w,
		BR:                          c.buf.Reader,
		ReadLimit:                   -1,
		CompressionLevel:            websocket.DisableCompression,
		IsServer:                    false,
		NegotiatedPerMessageDeflate: false,
	}
	c.mu.Unlock()
	return nil
}

func (c *Client) Connect(ctx context.Context, uri *url.URL, bootstrap string, dial ConnectFn) (*types.RPCResponse, error) {
	id := nextId.Add(1)
	if err := c.connect(ctx, uri, bootstrap, dial); err != nil {
		return nil, fmt.Errorf("%d: %w", id, err)
	}

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
	if c.conn.Conn == nil {
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
	for i, l := range c.listenerFixed {
		if l.name == "" {
			c.listenerFixed[i] = listener{
				name: name,
				fn:   fn,
			}
			return
		}
	}
	c.listenerExtra = append(c.listenerExtra, listener{
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
	if c.conn.Conn == nil {
		return errors.New("closed")
	}

	c.nextCB++
	if len(c.callbacks) == 0 {
		if c.callbacks == nil {
			c.callbacks = make(map[types.Callback]func(response types.RPCResponse), 1)
		}
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
	if c.conn.Conn == nil {
		return errors.New("closed")
	}
	c.buf.ReadBuffer.Reset()
	if _, _, err := c.conn.NextReadIntoBuffer(c.buf.ReadBuffer); err != nil {
		return err
	}
	res := types.RPCResponse{}
	if err := res.FastUnmarshalJSON(c.buf.ReadBuffer.Bytes()); err != nil {
		return err
	}
	matched := false
	if res.Callback != 0 {
		matched = true
		c.callbacks[res.Callback](res)
		delete(c.callbacks, res.Callback)
	} else if res.Name != "" {
		for _, l := range c.listenerFixed {
			if l.name == res.Name {
				l.fn(res)
				matched = true
			}
		}
		for _, l := range c.listenerExtra {
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

var (
	readBufferPool sync.Pool

	writeBufferPool = sync.Pool{New: func() any {
		return &writeBuf{p: make([]byte, 1024)}
	}}
)

type writeBuf struct {
	p []byte
}

type readBuffer struct {
	*bufio.Reader
	ReadBuffer *bytes.Buffer
}

func newReadBuffer(r io.Reader) *readBuffer {
	if v := readBufferPool.Get(); v != nil {
		br := v.(*readBuffer)
		br.Reader.Reset(r)
		return br
	}
	return &readBuffer{
		Reader:     bufio.NewReaderSize(r, 2048),
		ReadBuffer: bytes.NewBuffer(make([]byte, 0, 4096)),
	}
}

func putReadBuffer(br *readBuffer) {
	br.Reader.Reset(nil)
	readBufferPool.Put(br)
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
