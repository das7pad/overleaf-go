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
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/textproto"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

func New(handler http.HandlerFunc) *WSServer {
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
	h     http.HandlerFunc
	l     net.Listener
	mu    sync.Mutex
}

func (s *WSServer) SetStatus(ok bool) {
	s.ok.Store(ok)
}

func (s *WSServer) Shutdown(ctx context.Context) error {
	s.ok.Store(false)
	s.mu.Lock()
	if s.state.CompareAndSwap(stateRunning, stateClosing) {
		_ = s.l.Close()
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
	if !s.state.CompareAndSwap(stateDead, stateRunning) {
		return http.ErrServerClosed
	}
	s.l = l
	s.mu.Unlock()

	var errDelay time.Duration
	for s.state.Load() == stateRunning {
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
		wc := wsConn{bufferedConn: &bufferedConn{Conn: c}}
		go wc.serve(s.h, s.decrementN, s.ok.Load)
	}
	return http.ErrServerClosed
}

func (s *WSServer) decrementN() {
	s.n.Add(^uint32(0))
}

type wsConn struct {
	*bufferedConn
	reads    uint8
	ok       bool
	hijacked bool
}

type bufferedConn struct {
	net.Conn
	brw *bufio.ReadWriter
}

func (c *bufferedConn) ReleaseBuffers() {
	if c.brw != nil {
		putReader(c.brw)
		c.brw = nil
	}
}

func (c *wsConn) WriteTimeout(p []byte, d time.Duration) (int, error) {
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

func (c *wsConn) serve(h http.HandlerFunc, deref func(), ok func() bool) {
	defer deref()
	defer func() {
		if !c.hijacked {
			_ = c.Close()
			c.ReleaseBuffers()
		}
	}()
	c.brw = newReader(c, c.Conn)

	if c.SetReadDeadline(time.Now().Add(30*time.Second)) != nil {
		return
	}
	for {
		err := c.nextRequest(h, ok)
		if err != nil {
			if e, ok := err.(httpStatusError); ok {
				_, _ = c.WriteTimeout(e.Response(), 5*time.Second)
			}
			return
		}
		if c.hijacked {
			return
		}
		if c.SetReadDeadline(time.Now().Add(15*time.Second)) != nil {
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

func (c *wsConn) nextRequest(fn http.HandlerFunc, ok func() bool) error {
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
		return c.handleWsRequest(fn, err)
	}
	return httpStatusError(http.StatusBadRequest)
}

func (c *wsConn) handleStatusRequest(ok func() bool) error {
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
		_, err = c.Write(response200)
	} else {
		_, err = c.Write(response503)
	}
	return err
}

func (c *wsConn) handleWsRequest(fn http.HandlerFunc, err error) error {
	tp := newTPReader(c.brw.Reader)
	h, err := tp.ReadMIMEHeader()
	putTPReader(tp)

	if err != nil || c.brw.Reader.Buffered() > 0 {
		if err == errTooManyReads || err == bufio.ErrBufferFull {
			return httpStatusError(http.StatusRequestURITooLong)
		}
		if os.IsTimeout(err) {
			return httpStatusError(http.StatusRequestTimeout)
		}
		return httpStatusError(http.StatusBadRequest)
	}

	c.brw.Reader.Reset(c.Conn)
	r := http.Request{
		Header: http.Header(h),
		Method: http.MethodGet,
	}
	fn(&wsResponseWriter{wsConn: c}, &r)
	return nil
}

type wsResponseWriter struct {
	*wsConn
	h             http.Header
	headerWritten bool
}

func (c *wsResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	c.hijacked = true
	return c.bufferedConn, c.brw, nil
}

func (c *wsResponseWriter) Header() http.Header {
	if c.h == nil {
		c.h = make(http.Header)
	}
	return c.h
}

func (c *wsResponseWriter) Write(p []byte) (int, error) {
	if !c.headerWritten {
		c.WriteHeader(http.StatusOK)
	}
	return c.WriteTimeout(p, 10*time.Second)
}

func (c *wsResponseWriter) WriteHeader(code int) {
	c.headerWritten = true
	if c.SetWriteDeadline(time.Now().Add(10*time.Second)) != nil {
		_ = c.Close()
		return
	}
	_, _ = c.brw.WriteString("HTTP/1.1 ")
	_, _ = c.brw.WriteString(strconv.FormatInt(int64(code), 10))
	_ = c.brw.WriteByte(' ')
	if text := http.StatusText(code); text != "" {
		_, _ = c.brw.WriteString(text)
	} else {
		_, _ = c.brw.WriteString("unknown status")
	}
	_, _ = c.brw.WriteString("\r\n")
	_ = c.h.Write(c.brw)
	_, _ = c.brw.WriteString("\r\n")
	if err := c.brw.Flush(); err != nil {
		_ = c.Close()
	}
}

var rwPool sync.Pool

func newReader(r io.Reader, w io.Writer) *bufio.ReadWriter {
	if v := rwPool.Get(); v != nil {
		br := v.(*bufio.ReadWriter)
		br.Reader.Reset(r)
		br.Writer.Reset(w)
		return br
	}
	return bufio.NewReadWriter(
		bufio.NewReaderSize(r, 2048),
		bufio.NewWriterSize(w, 2048),
	)
}

func putReader(br *bufio.ReadWriter) {
	br.Reader.Reset(nil)
	br.Writer.Reset(nil)
	rwPool.Put(br)
	return
}

var tpReaderPool sync.Pool

func newTPReader(br *bufio.Reader) *textproto.Reader {
	if v := tpReaderPool.Get(); v != nil {
		tr := v.(*textproto.Reader)
		tr.R = br
		return tr
	}
	return textproto.NewReader(br)
}

func putTPReader(r *textproto.Reader) {
	r.R = nil
	tpReaderPool.Put(r)
}
