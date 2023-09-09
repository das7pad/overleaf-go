// Golang port of Overleaf
// Copyright (C) 2021-2023 Jakob Ackermann <das7pad@outlook.com>
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
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

// watchingManager is safe for concurrent rendering and watching.
//
// "safe" in regard to simple data-races (read vs write on .assets during load)
// and logic data-races (read vs read on .assets before/after it was rewritten
// during the processing of a single HTTP request).
type watchingManager struct {
	*manager
}

type buildNotification struct {
	Manifest json.RawMessage `json:"manifest"`
}

func (wm *watchingManager) watch(cdnURL sharedTypes.URL) {
	log.Println("assets: watch: waiting for rebuilds")
	u := cdnURL.
		WithPath("/event-source").
		WithQuery(url.Values{"manifest": {"true"}}).
		String()
	for {
		res, err := http.Get(u)
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
			if blob != "event: rebuild" {
				continue
			}
			if !r.Scan() {
				break
			}
			bn := buildNotification{}
			err = json.Unmarshal(r.Bytes()[len("data: "):], &bn)
			if err != nil {
				log.Printf(
					"assets: watch: bad rebuild notification %q", err.Error(),
				)
				continue
			}
			log.Println("assets: watch: reloading")
			err = wm.loadFrom(bytes.NewReader(bn.Manifest))
			if err != nil {
				log.Printf(
					"assets: watch: reload failed: %q", err.Error(),
				)
				continue
			}
		}
		log.Printf(
			"assets: watch: streaming stopped with err=%v", r.Err(),
		)
		_ = res.Body.Close()
	}
}

func (wm *watchingManager) RenderingStart() {
	wm.mu.RLock()
}

func (wm *watchingManager) RenderingEnd() {
	wm.mu.RUnlock()
}
