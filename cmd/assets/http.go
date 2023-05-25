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
	"time"
)

func serve(addr string, o *outputCollector) {
	epoch := time.Now()
	err := http.ListenAndServe(addr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/event-source") {
			handleEventSource(w, r, epoch, o)
		} else {
			blob, ok := o.GET(strings.TrimPrefix(r.URL.Path, "/"))
			if ok {
				ct := mime.TypeByExtension(path.Ext(r.URL.Path))
				if s := r.Header.Get("Origin"); s != "" {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				}
				w.Header().Set("Content-Type", ct)
				_, _ = w.Write(blob)
			} else {
				log.Printf("GET %s 404", r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}
		}
	}))
	if err != nil {
		panic(err)
	}
}
