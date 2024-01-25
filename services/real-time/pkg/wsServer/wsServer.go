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
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Handler func(c net.Conn, brw *RWBuffer, t0 time.Time, parseRequest func(parseJWT func([]byte)) error) error
type ClaimParser[T any] func([]byte) (T, error)

func New(handler Handler) *WSServer {
	srv := WSServer{
		h: handler,
	}
	srv.ok.Store(true)
	return &srv
}

type WSServer struct {
	state atomic.Uint32
	n     atomic.Uint32
	ok    atomic.Bool
	l     []net.Listener
	mu    sync.Mutex
	h     Handler
}

func (s *WSServer) SetStatus(ok bool) {
	s.ok.Store(ok)
}

func (s *WSServer) Shutdown(ctx context.Context) error {
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

func (s *WSServer) Serve(l net.Listener) error {
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
		wc := wsConn{BufferedConn: &BufferedConn{Conn: c}}
		go wc.serve(s.h, s.decrementN, s.ok.Load)
	}
}

func (s *WSServer) decrementN() {
	s.n.Add(^uint32(0))
}

type BufferedConn struct {
	net.Conn
	brw    *RWBuffer
	closed atomic.Bool
}

var errConnAlreadyClosed = errors.New("wsServer: connection already closed")

func (c *BufferedConn) Close() error {
	if c.closed.CompareAndSwap(false, true) {
		return c.Conn.Close()
	} else {
		return errConnAlreadyClosed
	}
}

func (c *BufferedConn) ReleaseBuffers() {
	if c.brw != nil {
		putBuffer(c.brw)
		c.brw = nil
	}
}

type wsConn struct {
	*BufferedConn
	reads       uint8
	hijacked    bool
	noKeepalive bool
}

func (c *wsConn) writeTimeout(p []byte, d time.Duration) (int, error) {
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
	requestLineStatusGET  = []byte("GET /status HTTP/1.1\r\n")
	requestLineStatusHEAD = []byte("HEAD /status HTTP/1.0\r\n")
	requestLineWS         = []byte("GET /socket.io HTTP/1.1\r\n")

	httpErrorHeaders = "\r\nConnection: close\r\nContent-Length: 0\r\n\r\n"
	response400      = []byte("HTTP/1.1 400 Bad Request" + httpErrorHeaders)
	response408      = []byte("HTTP/1.1 408 Request Timeout" + httpErrorHeaders)
	response414      = []byte("HTTP/1.1 414 Request URI Too Long" + httpErrorHeaders)
	response431      = []byte("HTTP/1.1 431 Request Header Fields Too Large" + httpErrorHeaders)

	response200 = []byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n")
	response503 = []byte("HTTP/1.1 503 Service Unavailable\r\nContent-Length: 0\r\n\r\n")

	errTooManyReads = errors.New("too many reads")
)

func (c *wsConn) serve(h Handler, deref func(), isOK func() bool) {
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
		err := c.nextRequest(h, now, isOK)
		if err != nil {
			if e, ok := err.(httpStatusError); ok {
				_, _ = c.writeTimeout(e.Response(), 5*time.Second)
			}
			return
		}
		if c.hijacked || c.noKeepalive {
			return
		}
		if c.SetDeadline(time.Now().Add(15*time.Second)) != nil {
			return
		}
		c.reads = 0
	}
}

func (c *wsConn) Read(p []byte) (int, error) {
	if c.reads >= 2 {
		return 0, errTooManyReads
	}
	c.reads += 1
	return c.Conn.Read(p)
}

func (c *wsConn) nextRequest(fn Handler, now time.Time, isOK func() bool) error {
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
	if bytes.Equal(l, requestLineWS) {
		return c.handleWsRequest(fn, now)
	}
	if bytes.Equal(l, requestLineStatusHEAD) {
		c.noKeepalive = true
		return c.handleStatusRequest(isOK)
	}
	if bytes.Equal(l, requestLineStatusGET) {
		return c.handleStatusRequest(isOK)
	}
	return httpStatusError(http.StatusBadRequest)
}

func (c *wsConn) handleStatusRequest(isOK func() bool) error {
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
	if isOK() {
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
	responseBodyStart       = []byte("\r\n\r\n")
)

func (c *wsConn) handleWsRequest(fn Handler, t0 time.Time) error {
	return fn(c.BufferedConn, c.brw, t0, c.parseWsRequest)
}

func (c *wsConn) parseWsRequest(parseJWT func([]byte)) error {
	checks := [6]bool{}
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
				if b, ok2 := bytes.CutSuffix(next, headerValueWSProtocolBS); ok2 {
					checks[4] = true
					parseJWT(b)
				}
			}
		case !checks[5] && len(value) == 24 && bytes.EqualFold(name, headerKeyWSKey):
			if _, err = base64.StdEncoding.Decode(buf[0:18], value); err != nil {
				return httpStatusError(http.StatusBadRequest)
			}
			buf = append(buf, responseWS...)
			buf = appendSecWebSocketAccept(buf, value)
			buf = append(buf, responseBodyStart...)
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
	c.hijacked = true
	if _, err := c.Write(buf); err != nil {
		return err
	}
	c.brw.Reader.Reset(c.Conn)
	return nil
}

func HTTPUpgrade(w http.ResponseWriter, r *http.Request, parseJWT func([]byte)) (net.Conn, *bufio.ReadWriter, error) {
	conn, brw, err := tryHTTPUpgrade(w, r, parseJWT)
	if err != nil {
		if code, ok := err.(httpStatusError); ok {
			w.WriteHeader(int(code))
		}
		return nil, brw, err
	}
	return conn, brw, nil
}

func tryHTTPUpgrade(w http.ResponseWriter, r *http.Request, parseJWT func([]byte)) (net.Conn, *bufio.ReadWriter, error) {
	h := r.Header
	ok := false
	for _, v := range h["Connection"] {
		var next string
		for !ok && len(v) > 0 {
			next, v, _ = strings.Cut(v, ",")
			v = strings.TrimSpace(v)
			if strings.EqualFold(next, "Upgrade") {
				ok = true
			}
		}
	}
	if !ok {
		return nil, nil, httpStatusError(http.StatusBadRequest)
	}
	if u := h["Upgrade"]; len(u) == 0 || !strings.EqualFold(u[0], "websocket") {
		return nil, nil, httpStatusError(http.StatusBadRequest)
	}
	if u := h["Sec-Websocket-Version"]; len(u) == 0 || !strings.EqualFold(u[0], "13") {
		return nil, nil, httpStatusError(http.StatusBadRequest)
	}
	ok = false
	jwtParsed := false
	for _, v := range h["Sec-Websocket-Protocol"] {
		var next string
		for len(v) > 0 {
			next, v, _ = strings.Cut(v, ",")
			v = strings.TrimSpace(v)
			if strings.EqualFold(next, "v8.real-time.overleaf.com") {
				ok = true
				continue
			}
			if jwtParsed {
				continue
			}
			if b, ok2 := strings.CutSuffix(next, ".bootstrap.v8.real-time.overleaf.com"); ok2 {
				jwtParsed = true
				parseJWT([]byte(b))
			}
		}
	}
	if !ok || !jwtParsed {
		return nil, nil, httpStatusError(http.StatusBadRequest)
	}
	if k := h["Sec-Websocket-Key"]; len(k) != 1 || len(k[0]) != 24 {
		return nil, nil, httpStatusError(http.StatusBadRequest)
	}
	key := []byte(h["Sec-Websocket-Key"][0])
	{
		buf := [18]byte{}
		if _, err := base64.StdEncoding.Decode(buf[0:18], key); err != nil {
			return nil, nil, httpStatusError(http.StatusBadRequest)
		}
	}

	c, brw, err := w.(http.Hijacker).Hijack()
	if err != nil {
		return nil, nil, err
	}

	if brw.Reader.Buffered() > 0 {
		return nil, nil, httpStatusError(http.StatusBadRequest)
	}

	buf := brw.AvailableBuffer()
	buf = append(buf, responseWS...)
	buf = appendSecWebSocketAccept(buf, key)
	buf = append(buf, responseBodyStart...)

	if _, err = c.Write(buf); err != nil {
		_ = c.Close()
		return nil, nil, err
	}

	return c, brw, nil
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
