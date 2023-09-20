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
	"bufio"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

type BuildNotification struct {
	Manifest json.RawMessage  `json:"manifest"`
	Errors   []minimalMessage `json:"errors"`
	Warnings []minimalMessage `json:"warnings"`
	Name     string           `json:"name"`
}

type minimalLocation struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Column     int    `json:"column"`
	Length     int    `json:"length"`
	LineText   string `json:"lineText"`
	Suggestion string `json:"suggestion"`
}

type minimalMessage struct {
	Text     string          `json:"text"`
	Location minimalLocation `json:"location"`
}

func convertMessages(in []api.Message) []minimalMessage {
	out := make([]minimalMessage, 0, len(in))
	for _, m := range in {
		if m.Location == nil {
			m.Location = &api.Location{}
		}
		out = append(out, minimalMessage{
			Text: m.Text,
			Location: minimalLocation{
				File:       m.Location.File,
				Line:       m.Location.Line,
				Column:     m.Location.Column,
				Length:     m.Location.Length,
				LineText:   m.Location.LineText,
				Suggestion: m.Location.Suggestion,
			},
		})
	}
	return out
}

func (o *outputCollector) AddListener(c chan BuildNotification) func() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.onBuild = append(o.onBuild, c)
	return func() {
		o.mu.Lock()
		for i, c2 := range o.onBuild {
			if c == c2 {
				o.onBuild[i] = o.onBuild[len(o.onBuild)-1]
				o.onBuild = o.onBuild[:len(o.onBuild)-1]
			}
		}
		o.mu.Unlock()
		close(c)
		for range c {
		}
	}
}

func (o *outputCollector) notifyAboutBuild(name string, r *api.BuildResult) error {
	b := BuildNotification{
		Name:     name,
		Errors:   convertMessages(r.Errors),
		Warnings: convertMessages(r.Warnings),
	}

	o.mu.Lock()
	b.Manifest = o.mem["manifest.json"]
	for _, f := range o.onBuild {
		f <- b
	}
	o.mu.Unlock()
	return nil
}

func (o *outputCollector) handleEventSource(w http.ResponseWriter, r *http.Request) {
	c := make(chan BuildNotification, 10)
	defer o.AddListener(c)()
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	conn, buf, err := w.(http.Hijacker).Hijack()
	if err != nil {
		log.Println("frontend: event-source: hijack", err)
		return
	}
	defer func() {
		_ = conn.Close()
	}()

	disconnected := make(chan struct{})
	go func() {
		_, _ = conn.Read(make([]byte, 1))
		close(disconnected)
	}()

	blob, _ := o.Get("manifest.json")
	c <- BuildNotification{
		Name:     "initial",
		Manifest: blob,
		Errors:   make([]minimalMessage, 0),
		Warnings: make([]minimalMessage, 0),
	}

	for err == nil {
		select {
		case <-disconnected:
			return
		case m := <-c:
			if blob, err = json.Marshal(m); err == nil {
				err = writeSSE(buf, "rebuild", blob)
			}
		}
	}
	if err != nil && !strings.Contains(err.Error(), "broken pipe") {
		select {
		case <-disconnected:
			return
		default:
			log.Println("frontend: event-source: rebuild", err)
		}
	}
}

func writeSSE(w *bufio.ReadWriter, event string, data []byte) error {
	if _, err := w.Write([]byte("event: " + event + "\n")); err != nil {
		return err
	}
	if _, err := w.Write([]byte("data: ")); err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	if _, err := w.Write([]byte("\n\n\n")); err != nil {
		return err
	}
	if err := w.Flush(); err != nil {
		return err
	}
	return nil
}
