// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

type RouterOptions struct {
	Ready func() bool
}

type appendOnlyCtx = context.Context

type Context struct {
	appendOnlyCtx
	Writer  http.ResponseWriter
	Request *http.Request
	t0      time.Time
}

func (c *Context) T0() time.Time {
	return c.t0
}

func (c *Context) Param(key string) string {
	return mux.Vars(c.Request)[key]
}

func (c Context) AddValue(key interface{}, v interface{}) *Context {
	c.appendOnlyCtx = context.WithValue(c.appendOnlyCtx, key, v)
	return &c
}

// ClientIP returns the last IP of the last X-Forwarded-For header on the
// request, if any, else it returns the requests .RemoteAddr.
func (c *Context) ClientIP() string {
	ip := ""
	for _, s := range c.Request.Header.Values("X-Forwarded-For") {
		ip = strings.TrimSpace(s[strings.LastIndexByte(s, ',')+1:])
	}
	if ip == "" {
		addr := c.Request.RemoteAddr
		ip = addr[:strings.LastIndexByte(addr, ':')]
	}
	return ip
}

type HandlerFunc func(c *Context)

func (f HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := Context{
		appendOnlyCtx: r.Context(),
		Writer:        w,
		Request:       r,
		t0:            time.Now(),
	}
	w.Header().Set("Cache-Control", "no-store")
	f(&c)
}

type MiddlewareFunc func(next HandlerFunc) HandlerFunc

type Router struct {
	*mux.Router
	middlewares []MiddlewareFunc
}

func (r *Router) Use(fns ...MiddlewareFunc) {
	r.middlewares = append(r.middlewares, fns...)
}

func (r *Router) wrap(f HandlerFunc) HandlerFunc {
	for i := len(r.middlewares) - 1; i >= 0; i-- {
		f = r.middlewares[i](f)
	}
	return f
}

func (r *Router) NoRoute(f HandlerFunc) {
	r.NotFoundHandler = r.wrap(f)
}

func (r *Router) DELETE(endpoint string, f HandlerFunc) {
	r.NewRoute().Methods(http.MethodDelete).Path(endpoint).Handler(r.wrap(f))
}

func (r *Router) GET(endpoint string, f HandlerFunc) {
	r.NewRoute().Methods(http.MethodGet).Path(endpoint).Handler(r.wrap(f))
}

func (r *Router) HEAD(endpoint string, f HandlerFunc) {
	r.NewRoute().Methods(http.MethodHead).Path(endpoint).Handler(r.wrap(f))
}

func (r *Router) POST(endpoint string, f HandlerFunc) {
	r.NewRoute().Methods(http.MethodPost).Path(endpoint).Handler(r.wrap(f))
}

func (r *Router) PUT(endpoint string, f HandlerFunc) {
	r.NewRoute().Methods(http.MethodPut).Path(endpoint).Handler(r.wrap(f))
}

func (r *Router) Group(partial string) *Router {
	middlewares := make([]MiddlewareFunc, len(r.middlewares))
	copy(middlewares, r.middlewares)
	var r2 *mux.Router
	if partial == "" {
		// Skip traversing into an extra sub-router.
		r2 = r.Router
	} else {
		r2 = r.Router.PathPrefix(partial).Subrouter()
	}
	return &Router{
		Router:      r2,
		middlewares: middlewares,
	}
}

func NewRouter(options *RouterOptions) *Router {
	router := Router{
		Router: mux.NewRouter(),
	}
	router.OmitRouteFromContext(true)
	statusHandler := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
	if options.Ready != nil {
		statusHandler = func(w http.ResponseWriter, _ *http.Request) {
			if options.Ready() {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusServiceUnavailable)
			}
		}
	}
	router.NewRoute().
		MatcherFunc(func(r *http.Request, _ *mux.RouteMatch) bool {
			return (r.Method == http.MethodGet || r.Method == http.MethodHead) &&
				r.URL.Path == "/status"
		}).
		HandlerFunc(statusHandler)
	return &router
}
