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

package less

import (
	"encoding/base64"
	"encoding/json"
	"path"
	"path/filepath"
	"strings"

	"github.com/das7pad/overleaf-go/pkg/assets/pkg/frontendBuild/pkg/vlq"
)

var b64 = base64.StdEncoding

type sourceMapWriter struct {
	sourceMap
	lastToken    token
	lastTokenFId int16
	column       int32
	lastColumn   int32
	cssBuf       strings.Builder
	sizeEstimate int
	dir          string
	fIdMap       map[int16]int16
}

func newSourceMapWriter(f string) *sourceMapWriter {
	dir := path.Dir(f)
	return &sourceMapWriter{
		sourceMap: sourceMap{
			Version:        3,
			Sources:        make([]string, 0, 150),
			SourcesContent: make([]string, 0, 150),
		},
		dir:    dir,
		fIdMap: make(map[int16]int16),
	}
}

func (w *sourceMapWriter) SetContent(f string, fId int16, s string) {
	w.fIdMap[fId] = int16(len(w.Sources))
	p, _ := filepath.Rel(w.dir, f)
	w.Sources = append(w.Sources, p)
	w.SourcesContent = append(w.SourcesContent, s)

	w.sizeEstimate += len(s) + len(s)/3
}

func (w *sourceMapWriter) StartWriting() {
	w.cssBuf.Grow(w.sizeEstimate)
	w.Mappings = make([]byte, 0, w.sizeEstimate)
	w.Mappings = append(w.Mappings, '"')
}

func (w *sourceMapWriter) FinishWriting() {
	w.Mappings = append(w.Mappings, '"')
}

func (w *sourceMapWriter) CSS() string {
	return w.cssBuf.String()
}

type sourceMap struct {
	Version        int             `json:"version"`
	Sources        []string        `json:"sources"`
	SourcesContent []string        `json:"sourcesContent"`
	Mappings       json.RawMessage `json:"mappings"`
}

func (w *sourceMapWriter) SourceMap() (string, error) {
	blob, err := json.Marshal(w.sourceMap)
	return string(blob), err
}

func (w *sourceMapWriter) WriteString(s string) {
	w.WriteToken(token{v: s})
}

func (w *sourceMapWriter) WriteTokens(tt tokens) {
	for _, t := range tt {
		w.WriteToken(t)
	}
}

func (w *sourceMapWriter) WriteToken(t token) {
	buf := w.Mappings

	if t.kind == tokenNewline {
		if n := len(buf); n > 1 && buf[len(buf)-1] != ';' {
			buf = append(buf, ',')
			buf = vlq.Encode(buf, w.column-w.lastColumn)
		}
		for i := 0; i < len(t.v); i++ {
			buf = append(buf, ';')
		}
		w.column = 0
		w.lastColumn = 0
	} else {
		if n := len(buf); n > 1 && buf[len(buf)-1] != ';' {
			buf = append(buf, ',')
		}
		buf = vlq.Encode(buf, w.column-w.lastColumn)
		if t.f != 0 {
			fId := w.fIdMap[t.f]
			buf = vlq.Encode(buf, int32(fId-w.lastTokenFId))
			buf = vlq.Encode(buf, int32(t.line-w.lastToken.line))
			buf = vlq.Encode(buf, int32(t.column-w.lastToken.column))
			w.lastToken = t
			w.lastTokenFId = fId
		}
		w.lastColumn = w.column
		w.column += int32(len(t.v))
	}
	w.cssBuf.WriteString(t.v)
	w.Mappings = buf
}

func InlineSourceMap(s string, srcMap string) string {
	prefix := "\n/*# sourceMappingURL=data:application/json;base64,"
	suffix := " */"
	srcMapBuf := []byte(srcMap)
	srcMapLen := b64.EncodedLen(len(srcMapBuf))
	buf := make([]byte, 0+
		len(s)+
		len(prefix)+
		srcMapLen+
		len(suffix),
	)
	idx := 0
	idx += copy(buf[idx:], s)
	idx += copy(buf[idx:], prefix)
	b64.Encode(buf[idx:], srcMapBuf)
	idx += srcMapLen
	copy(buf[idx:], suffix)
	return string(buf)
}
