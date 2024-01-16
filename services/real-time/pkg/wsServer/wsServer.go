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

package wsServer

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type Handler[T any] func(c net.Conn, brw *RWBuffer, t0 time.Time, claims T, jwtErr error)
type ClaimParser[T any] func([]byte) (T, error)

func New[T any](handler Handler[T], parseClaims ClaimParser[T]) *WSServer[T] {
	srv := WSServer[T]{
		h:           handler,
		parseClaims: parseClaims,
	}
	srv.ok.Store(true)
	return &srv
}

type WSServer[T any] struct {
	state       atomic.Uint32
	n           atomic.Uint32
	ok          atomic.Bool
	l           []net.Listener
	mu          sync.Mutex
	h           Handler[T]
	parseClaims ClaimParser[T]
}

func (s *WSServer[T]) SetStatus(ok bool) {
	s.ok.Store(ok)
}

func (s *WSServer[T]) Shutdown(ctx context.Context) error {
	s.ok.Store(false)
	s.mu.Lock()
	if s.state.CompareAndSwap(stateRunning, stateClosing) {
		for _, l := range s.l {
			_ = l.Close()
		}
	}
	s.mu.Unlock()

	for s.n.Load() > 0 && ctx.Err() == nil {
		time.Sleep(10 * time.Millisecond)
	}
	if s.n.Load() == 0 {
		s.state.Store(stateClosed)
	}
	return ctx.Err()
}

const (
	stateDead = iota
	stateRunning
	stateClosing
	stateClosed
)

func (s *WSServer[T]) Serve(l net.Listener) error {
	s.mu.Lock()
	if !(s.state.Load() == stateRunning ||
		s.state.CompareAndSwap(stateDead, stateRunning)) {
		s.mu.Unlock()
		return http.ErrServerClosed
	}
	s.l = append(s.l, l)
	s.mu.Unlock()

	var errDelay time.Duration
	for {
		c, err := l.Accept()
		if err != nil {
			if s.state.Load() != stateRunning {
				return http.ErrServerClosed
			}
			if e, ok := err.(net.Error); ok && e.Temporary() {
				if errDelay == 0 {
					errDelay = 5 * time.Millisecond
				}
				errDelay *= 2
				if errDelay > time.Second {
					errDelay = time.Second
				}
				log.Printf(
					"wsServer: Accept error: %v; retrying in  %s",
					err, errDelay,
				)
				time.Sleep(errDelay)
				continue
			}
			return err
		}
		s.n.Add(1)
		errDelay = 0
		wc := wsConn[T]{BufferedConn: &BufferedConn{Conn: c}}
		go wc.serve(s.h, s.parseClaims, s.decrementN, s.ok.Load)
	}
}

func (s *WSServer[T]) decrementN() {
	s.n.Add(^uint32(0))
}

type BufferedConn struct {
	net.Conn
	brw *RWBuffer
}

func (c *BufferedConn) ReleaseBuffers() {
	if c.brw != nil {
		putBuffer(c.brw)
		c.brw = nil
	}
}

type wsConn[T any] struct {
	*BufferedConn
	reads    uint8
	hijacked bool
}

func (c *wsConn[T]) writeTimeout(p []byte, d time.Duration) (int, error) {
	if err := c.SetWriteDeadline(time.Now().Add(d)); err != nil {
		_ = c.Close()
		return 0, err
	}
	return c.Write(p)
}

type httpStatusError int

func (e httpStatusError) Error() string {
	return ""
}

func (e httpStatusError) Response() []byte {
	switch e {
	case http.StatusBadRequest:
		return response400
	case http.StatusRequestTimeout:
		return response408
	case http.StatusRequestURITooLong:
		return response414
	case http.StatusRequestHeaderFieldsTooLarge:
		return response431
	default:
		return nil
	}
}

var (
	requestLineStatus = []byte("GET /status HTTP/1.1\r\n")
	requestLineWS     = []byte("GET /socket.io HTTP/1.1\r\n")

	httpErrorHeaders = "\r\nConnection: close\r\nContent-Length: 0\r\n\r\n"
	response400      = []byte("HTTP/1.1 400 Bad Request" + httpErrorHeaders)
	response408      = []byte("HTTP/1.1 408 Request Timeout" + httpErrorHeaders)
	response414      = []byte("HTTP/1.1 414 Request URI Too Long" + httpErrorHeaders)
	response431      = []byte("HTTP/1.1 431 Request Header Fields Too Large" + httpErrorHeaders)

	response200 = []byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n")
	response503 = []byte("HTTP/1.1 503 Service Unavailable\r\nContent-Length: 0\r\n\r\n")

	errTooManyReads = errors.New("too many reads")
)

func (c *wsConn[T]) serve(h Handler[T], parseClaims ClaimParser[T], deref func(), ok func() bool) {
	defer deref()
	defer func() {
		if !c.hijacked {
			_ = c.Close()
			c.ReleaseBuffers()
		}
	}()
	c.brw = newBuffer(c)

	now := time.Now()
	if c.SetDeadline(now.Add(30*time.Second)) != nil {
		return
	}
	for {
		err := c.nextRequest(now, h, parseClaims, ok)
		if err != nil {
			if e, ok := err.(httpStatusError); ok {
				_, _ = c.writeTimeout(e.Response(), 5*time.Second)
			}
			return
		}
		if c.hijacked {
			return
		}
		if c.SetDeadline(time.Now().Add(15*time.Second)) != nil {
			return
		}
		c.reads = 0
	}
}

func (c *wsConn[T]) Read(p []byte) (int, error) {
	if c.reads >= 2 {
		return 0, errTooManyReads
	}
	c.reads += 1
	return c.Conn.Read(p)
}

func (c *wsConn[T]) nextRequest(now time.Time, fn Handler[T], claimParser ClaimParser[T], ok func() bool) error {
	l, err := c.brw.ReadSlice('\n')
	if err != nil {
		if err == errTooManyReads || err == bufio.ErrBufferFull {
			return httpStatusError(http.StatusRequestURITooLong)
		}
		if len(l) == 0 {
			return err
		}
		if os.IsTimeout(err) {
			return httpStatusError(http.StatusRequestTimeout)
		}
		return httpStatusError(http.StatusBadRequest)
	}
	if bytes.Equal(l, requestLineStatus) {
		return c.handleStatusRequest(ok)
	}
	if bytes.Equal(l, requestLineWS) {
		return c.handleWsRequest(now, fn, claimParser)
	}
	return httpStatusError(http.StatusBadRequest)
}

func (c *wsConn[T]) handleStatusRequest(ok func() bool) error {
	for {
		l, err := c.brw.ReadSlice('\n')
		if err != nil {
			if err == errTooManyReads || err == bufio.ErrBufferFull {
				return httpStatusError(http.StatusRequestHeaderFieldsTooLarge)
			}
			if os.IsTimeout(err) {
				return httpStatusError(http.StatusRequestTimeout)
			}
			return httpStatusError(http.StatusBadRequest)
		}
		if len(l) <= 2 {
			break
		}
	}

	var err error
	if ok() {
		_, err = c.writeTimeout(response200, 10*time.Second)
	} else {
		_, err = c.writeTimeout(response503, 10*time.Second)
	}
	return err
}

var (
	separatorColon          = []byte(":")
	separatorComma          = []byte(",")
	headerKeyConnection     = []byte("Connection")
	headerValueConnection   = []byte("Upgrade")
	headerKeyUpgrade        = []byte("Upgrade")
	headerValueUpgrade      = []byte("websocket")
	headerKeyWSVersion      = []byte("Sec-Websocket-Version")
	headerValueWSVersion    = []byte("13")
	headerKeyWSProtocol     = []byte("Sec-Websocket-Protocol")
	headerValueWSProtocol   = []byte("v8.real-time.overleaf.com")
	headerValueWSProtocolBS = []byte(".bootstrap.v8.real-time.overleaf.com")
	headerKeyWSKey          = []byte("Sec-Websocket-Key")
	responseWS              = []byte("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Protocol: v8.real-time.overleaf.com\r\nSec-WebSocket-Accept: ")
)

func (c *wsConn[T]) handleWsRequest(t0 time.Time, fn Handler[T], claimParser ClaimParser[T]) error {
	checks := [6]bool{}
	var jwt T
	var jwtErr error
	buf := c.brw.WriteBuffer[:0]
	for {
		l, err := c.brw.ReadSlice('\n')
		if err != nil {
			if err == errTooManyReads || err == bufio.ErrBufferFull {
				return httpStatusError(http.StatusRequestHeaderFieldsTooLarge)
			}
			if os.IsTimeout(err) {
				return httpStatusError(http.StatusRequestTimeout)
			}
			return httpStatusError(http.StatusBadRequest)
		}
		if len(l) <= 2 {
			break
		}
		name, value, ok := bytes.Cut(l, separatorColon)
		value = bytes.TrimSpace(value)
		if !ok || len(name) == 0 || len(value) == 0 {
			return httpStatusError(http.StatusBadRequest)
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
		case !checks[2] && bytes.EqualFold(name, headerKeyWSVersion):
			checks[2] = bytes.Equal(value, headerValueWSVersion)
		case !(checks[3] && checks[4]) && bytes.EqualFold(name, headerKeyWSProtocol):
			var next []byte
			for ok && len(value) > 0 {
				next, value, ok = bytes.Cut(value, separatorComma)
				value = bytes.TrimSpace(value)
				if !checks[3] && bytes.Equal(next, headerValueWSProtocol) {
					checks[3] = true
					continue
				}
				if checks[4] {
					continue // parse JWT once
				}
				if b, _, ok2 := bytes.Cut(next, headerValueWSProtocolBS); ok2 {
					checks[4] = true
					if jwt, jwtErr = claimParser(b); jwtErr != nil {
						continue
					}
				}
			}
		case !checks[5] && len(value) == 24 && bytes.EqualFold(name, headerKeyWSKey):
			if _, err = base64.StdEncoding.Decode(buf[0:18], value); err != nil {
				return httpStatusError(http.StatusBadRequest)
			}
			buf = append(buf, responseWS...)
			buf = appendSecWebSocketAccept(buf, value)
			checks[5] = true
		}
	}
	if c.brw.Reader.Buffered() > 0 {
		return httpStatusError(http.StatusBadRequest)
	}
	for _, ok := range checks {
		if !ok {
			return httpStatusError(http.StatusBadRequest)
		}
	}
	buf = append(buf, '\r', '\n', '\r', '\n')
	if _, err := c.Write(buf); err != nil {
		return err
	}

	c.hijacked = true
	c.brw.Reader.Reset(c.Conn)
	fn(c.BufferedConn, c.brw, t0, jwt, jwtErr)
	return nil
}

var wsKeyGUID = []byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11")

func appendSecWebSocketAccept(buf []byte, k []byte) []byte {
	h := sha1.New()
	h.Write(k)
	h.Write(wsKeyGUID)
	k = h.Sum(k[:0])
	n := base64.StdEncoding.EncodedLen(len(k))
	base64.StdEncoding.Encode(buf[len(buf):len(buf)+n], k)
	return buf[:len(buf)+n]
}

var bufferPool sync.Pool

func newBuffer(r io.Reader) *RWBuffer {
	if v := bufferPool.Get(); v != nil {
		br := v.(*RWBuffer)
		br.Reader.Reset(r)
		return br
	}
	return &RWBuffer{
		Reader:      bufio.NewReaderSize(r, 2048),
		WriteBuffer: make([]byte, 2048),
	}
}

func putBuffer(br *RWBuffer) {
	br.Reader.Reset(nil)
	bufferPool.Put(br)
	return
}

type RWBuffer struct {
	*bufio.Reader
	WriteBuffer []byte
}
