// Golang port of Overleaf
// Copyright (C) 2021 Jakob Ackermann <das7pad@outlook.com>
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

package assets

import (
	"bufio"
	"log"
	"net/http"
	"time"
)

func (m *manager) watch() {
	log.Println("assets: watch: waiting for rebuilds")
	for {
		res, err := http.Get(string(m.base) + "/event-source")
		if err != nil {
			time.Sleep(time.Second)
			log.Printf(
				"assets: watch: GET /event-source: %q",
				err.Error(),
			)
			continue
		}
		if status := res.StatusCode; status != 200 {
			time.Sleep(time.Second)
			log.Printf(
				"assets: watch: GET /event-source: unexpected status: %d",
				status,
			)
			_ = res.Body.Close()
			continue
		}
		if ct := res.Header.Get("Content-Type"); ct != "text/event-stream" {
			time.Sleep(time.Second)
			log.Printf(
				"assets: watch: GET /event-source: unexpected CT: %q", ct,
			)
			_ = res.Body.Close()
			continue
		}
		r := bufio.NewScanner(res.Body)
		r.Split(bufio.ScanLines)
		for r.Scan() {
			blob := r.Text()
			if blob != "event: epoch" && blob != "event: rebuild" {
				continue
			}
			if err = m.load(); err != nil {
				log.Printf(
					"assets: watch: reload failed: %q", err.Error(),
				)
				continue
			}
		}
		log.Printf(
			"assets: watch: streaming stopped with err=%q", r.Err(),
		)
		_ = res.Body.Close()
	}
}
