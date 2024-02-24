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

package router

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

	"github.com/das7pad/overleaf-go/pkg/jwt/projectJWT"
)

type Handler func(c net.Conn, brw *RWBuffer, t0 time.Time, parseRequest func(parseJWT func([]byte)) error) error
type ClaimParser[T any] func([]byte) (T, error)

type WSServer struct {
	state atomic.Uint32
	n     atomic.Uint32
	ok    atomic.Bool
	l     []net.Listener
	mu    sync.Mutex
	h     *httpController
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

func (s *WSServer) addListener(l net.Listener) error {
	s.mu.Lock()
	if !(s.state.Load() == stateRunning ||
		s.state.CompareAndSwap(stateDead, stateRunning)) {
		s.mu.Unlock()
		return http.ErrServerClosed
	}
	s.l = append(s.l, l)
	s.mu.Unlock()
	return nil
}

func (s *WSServer) handleAcceptError(err error, errDelay time.Duration) (time.Duration, error) {
	if s.state.Load() != stateRunning {
		return 0, http.ErrServerClosed
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
		return errDelay, nil
	}
	return 0, err
}

func (s *WSServer) Serve(l net.Listener) error {
	if ul, ok := l.(*net.UnixListener); ok {
		return s.ServeUnix(ul)
	}
	if err := s.addListener(l); err != nil {
		return err
	}

	var errDelay time.Duration
	for {
		c, err := l.Accept()
		if err != nil {
			if errDelay, err = s.handleAcceptError(err, errDelay); err != nil {
				return err
			}
			continue
		}
		s.n.Add(1)
		errDelay = 0
		go s.serve(c)
	}
}

func (s *WSServer) ServeUnix(l *net.UnixListener) error {
	if err := s.addListener(l); err != nil {
		return err
	}

	var errDelay time.Duration
	for {
		c, err := l.AcceptUnix()
		if err != nil {
			if errDelay, err = s.handleAcceptError(err, errDelay); err != nil {
				return err
			}
			continue
		}
		s.n.Add(1)
		errDelay = 0
		go s.serveUnix(c)
	}
}

func (s *WSServer) serve(conn net.Conn) {
	wc := wsConn{
		BufferedConn: &BufferedConn{Conn: conn},
		t0:           time.Now(),
		s:            s,
	}
	wc.serve()
}

func (s *WSServer) serveUnix(conn *net.UnixConn) {
	wc := wsConn{
		BufferedConn: &BufferedConn{Conn: conn},
		t0:           time.Now(),
		s:            s,
	}
	wc.serve()
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
	t0          time.Time
	s           *WSServer
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

func (c *wsConn) serve() {
	defer c.s.decrementN()
	defer func() {
		if !c.hijacked {
			_ = c.Close()
			c.ReleaseBuffers()
		}
	}()
	c.brw = newBuffer(c)

	if c.SetDeadline(c.t0.Add(30*time.Second)) != nil {
		return
	}
	for {
		err := c.nextRequest()
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

func (c *wsConn) nextRequest() error {
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
		return c.s.h.wsWsServer(c)
	}
	if bytes.Equal(l, requestLineStatusHEAD) {
		c.noKeepalive = true
		return c.handleStatusRequest()
	}
	if bytes.Equal(l, requestLineStatusGET) {
		return c.handleStatusRequest()
	}
	return httpStatusError(http.StatusBadRequest)
}

func (c *wsConn) handleStatusRequest() error {
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
	if c.s.ok.Load() {
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
	responseWS              = []byte("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Protocol: v8.real-time.overleaf.com\r\nSec-WebSocket-Accept: ")
	responseBodyStart       = []byte("\r\n\r\n")
)

func equalFoldASCII(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	return bytes.EqualFold(a, b)
}

func (c *wsConn) parseWsRequest(claims *projectJWT.Claims) (error, error) {
	checks := [6]bool{}
	buf := c.brw.WriteBuffer[:0]
	var jwtError error
	for {
		l, err := c.brw.ReadSlice('\n')
		if err != nil {
			if err == errTooManyReads || err == bufio.ErrBufferFull {
				return nil, httpStatusError(http.StatusRequestHeaderFieldsTooLarge)
			}
			if os.IsTimeout(err) {
				return nil, httpStatusError(http.StatusRequestTimeout)
			}
			return nil, httpStatusError(http.StatusBadRequest)
		}
		if len(l) <= 2 {
			break
		}
		name, value, ok := bytes.Cut(l, separatorColon)
		value = bytes.TrimSpace(value)
		if !ok || len(name) == 0 || len(value) == 0 {
			return nil, httpStatusError(http.StatusBadRequest)
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
		case !checks[2] && equalFoldASCII(name, headerKeyWSVersion):
			checks[2] = bytes.Equal(value, headerValueWSVersion)
		case !(checks[3] && checks[4]) && equalFoldASCII(name, headerKeyWSProtocol):
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
					jwtError = c.s.h.jwtProject.ParseInto(claims, b, c.t0)
				}
			}
		case !checks[5] && len(value) == 24 && equalFoldASCII(name, headerKeyWSKey):
			if _, err = base64.StdEncoding.Decode(buf[0:18], value); err != nil {
				return nil, httpStatusError(http.StatusBadRequest)
			}
			buf = append(buf, responseWS...)
			buf = appendSecWebSocketAccept(buf, value)
			buf = append(buf, responseBodyStart...)
			checks[5] = true
		}
	}
	if c.brw.Reader.Buffered() > 0 {
		return nil, httpStatusError(http.StatusBadRequest)
	}
	for _, ok := range checks {
		if !ok {
			return nil, httpStatusError(http.StatusBadRequest)
		}
	}
	c.hijacked = true
	if _, err := c.Write(buf); err != nil {
		return nil, err
	}
	c.brw.Reader.Reset(c.Conn)
	return jwtError, nil
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
	return base64.StdEncoding.AppendEncode(buf, k)
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
