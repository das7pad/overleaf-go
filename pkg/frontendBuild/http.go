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

package frontendBuild

import (
	"log"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
)

func (o *outputCollector) Get(p string) ([]byte, bool) {
	o.mu.Lock()
	blob, ok := o.mem[p]
	o.mu.Unlock()
	return blob, ok
}

func (o *outputCollector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
	} else if strings.HasSuffix(r.URL.Path, "/event-source") {
		o.handleEventSource(w, r)
	} else {
		p := strings.TrimPrefix(r.URL.Path, "/")
		p = strings.TrimPrefix(p, "assets/")
		blob, ok := o.Get(p)
		enc := r.Header.Get("Accept-Encoding")
		if ok &&
			o.preCompress != PreCompressNone &&
			strings.Contains(enc, "gzip") {
			preComp, gotPreComp := o.Get(p + ".gz")
			if gotPreComp {
				w.Header().Set("Content-Encoding", "gzip")
				blob = preComp
			}
		}
		if ok {
			exp := time.Now().UTC().Add(time.Hour).Format(http.TimeFormat)
			w.Header().Set("Expires", exp)
			ct := mime.TypeByExtension(path.Ext(r.URL.Path))
			w.Header().Set("Content-Type", ct)
			w.Header().Set("Content-Length", strconv.FormatInt(
				int64(len(blob)), 10,
			))
			switch r.Method {
			case http.MethodGet:
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(blob)
			case http.MethodHead:
				w.WriteHeader(http.StatusOK)
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		} else {
			log.Printf("GET %s 404", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}
}
