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

package translationsImport

import (
	"bytes"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

type cacheEntry struct {
	blob []byte
	s    string
}

type Cache struct {
	mu              sync.Mutex
	lastFindLocales time.Time
	cached          map[string]cacheEntry
	locales         []string
	imports         []string
	SourceDirs      []string
}

func (c *Cache) findLocalesWithCache(p string) ([]string, []string, map[string]cacheEntry, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Since(c.lastFindLocales) > time.Second {
		var err error
		before := c.locales
		c.locales, c.imports, err = FindLocales(path.Dir(p), c.SourceDirs)
		if err != nil {
			return nil, nil, nil, err
		}
		if strings.Join(before, "") != strings.Join(c.locales, "") {
			c.cached = make(map[string]cacheEntry, len(c.cached))
		}
		c.lastFindLocales = time.Now()
	}
	return c.locales, c.imports, c.cached, nil
}

func (c *Cache) ImportLng(p string, processLocale func(k, v string) string) (string, []string, error) {
	l, watch, cache, err := c.findLocalesWithCache(p)
	if err != nil {
		return "", watch, err
	}
	watch = append(
		[]string{path.Join(path.Dir(p), "en.json"), p},
		watch...,
	)
	srcBlob, err := os.ReadFile(p)
	if err != nil {
		return "", nil, err
	}
	{
		c.mu.Lock()
		e, ok := cache[p]
		c.mu.Unlock()
		if ok && bytes.Equal(srcBlob, e.blob) {
			return e.s, watch, err
		}
	}
	blob, err := ImportLng(p, l, processLocale)
	if err != nil {
		return "", watch, err
	}
	s := string(blob)
	c.mu.Lock()
	cache[p] = cacheEntry{blob: srcBlob, s: s}
	c.mu.Unlock()
	return s, watch, nil
}
