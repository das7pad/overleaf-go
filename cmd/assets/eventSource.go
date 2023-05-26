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
	"encoding/json"
	"log"
	"net/http"

	"github.com/evanw/esbuild/pkg/api"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type buildNotification struct {
	manifest []byte
	rebuild  []byte
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

type rebuildMessage struct {
	Errors   []minimalMessage `json:"errors"`
	Warnings []minimalMessage `json:"warnings"`
	Name     string           `json:"name"`
}

func convertMessages(in []api.Message) []minimalMessage {
	out := make([]minimalMessage, 0, len(in))
	for _, m := range in {
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

func (o *outputCollector) addListener(c chan buildNotification) func() {
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
	rebuild, err := json.Marshal(rebuildMessage{
		Name:     name,
		Errors:   convertMessages(r.Errors),
		Warnings: convertMessages(r.Warnings),
	})
	if err != nil {
		return errors.Tag(err, "serialize rebuild message")
	}

	o.mu.Lock()
	b := buildNotification{
		manifest: o.mem["manifest.json"],
		rebuild:  rebuild,
	}
	for _, f := range o.onBuild {
		f <- b
	}
	o.mu.Unlock()
	return nil
}

func (o *outputCollector) handleEventSource(w http.ResponseWriter, r *http.Request) {
	c := make(chan buildNotification, 10)
	defer o.addListener(c)()
	w.Header().Set("Content-Type", "text/event-stream")
	if err := writeSSE(w, "epoch", o.epochBlob); err != nil {
		log.Println(r.RequestURI, err)
		return
	}
	w.(http.Flusher).Flush()
	wantManifest := r.URL.Query().Get("manifest") == "true"
	for {
		select {
		case <-r.Context().Done():
			return
		case m := <-c:
			if err := writeSSE(w, "rebuild", m.rebuild); err != nil {
				log.Println(r.RequestURI, err)
				return
			}
			if wantManifest {
				if err := writeSSE(w, "manifest", m.manifest); err != nil {
					log.Println(r.RequestURI, err)
					return
				}
			}
			w.(http.Flusher).Flush()
		}
	}
}

func writeSSE(w http.ResponseWriter, event string, data []byte) error {
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
	return nil
}
