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

package main

import (
	"log"
	"mime"
	"net/http"
	"path"
	"strings"
)

func (o *outputCollector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/event-source") {
		o.handleEventSource(w, r)
	} else {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		blob, ok := o.GET(strings.TrimPrefix(r.URL.Path, "/"))
		if ok {
			ct := mime.TypeByExtension(path.Ext(r.URL.Path))
			w.Header().Set("Content-Type", ct)
			_, _ = w.Write(blob)
		} else {
			log.Printf("GET %s 404", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func (o *outputCollector) GET(p string) ([]byte, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	blob, ok := o.mem[p]
	return blob, ok
}
