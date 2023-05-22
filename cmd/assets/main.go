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
	"bytes"
	"compress/gzip"
	"fmt"
	"runtime"

	"github.com/evanw/esbuild/pkg/api"
	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func main() {
	p := os.Getenv("WEB_ROOT")
	watch := false
	concurrency := runtime.NumCPU()

	buf := bytes.NewBuffer(make([]byte, 0, 50_000_000))
	o := newOutputCollector(p, buf)

	eg := &errgroup.Group{}
	eg.SetLimit(concurrency)

	for _, options := range getConfigs(p) {
		cfg := options
		cfg.Plugins = append(cfg.Plugins, o.Plugin(cfg))
		eg.Go(func() error {
			c, ctxErr := api.Context(cfg.BuildOptions)
			if ctxErr != nil {
				return errors.Tag(ctxErr, cfg.Description)
			}
			if watch {
				if err := c.Watch(api.WatchOptions{}); err != nil {
					return errors.Tag(err, cfg.Description)
				}
			} else {
				t := &sharedTypes.Timed{}
				t.Begin()
				c.Rebuild()
				fmt.Println(cfg.Description, t.Stage())
				c.Dispose()
			}
			return nil
		})
	}

	total := &sharedTypes.Timed{}
	total.Begin()

	t := &sharedTypes.Timed{}
	t.Begin()
	if err := eg.Wait(); err != nil {
		panic(err)
	}
	fmt.Println("build", t.Stage())
	if err := writeStaticFiles(p, o); err != nil {
		panic(err)
	}
	fmt.Println("static", t.Stage())
	if err := o.Close(); err != nil {
		panic(err)
	}
	fmt.Println("close", t.Stage())

	// fmt.Println("assets")
	// for s, s2 := range o.manifest.Assets {
	// 	fmt.Println(s, "->", s2)
	// }
	// fmt.Println()
	// fmt.Println()
	// fmt.Println("entrypoints")
	// for s, strings := range o.manifest.EntryPoints {
	// 	fmt.Println(s, "->", strings)
	// }

	tarGz, err := compress(buf.Bytes())
	fmt.Println(len(tarGz), len(buf.Bytes()), err)
	fmt.Println("gz", t.Stage())

	fmt.Println(total.Stage())

	// err = os.WriteFile("/tmp/go.tar.gz", tarGz, 0o644)
	// fmt.Println(err)
}

func compress(blob []byte) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, len(blob)))
	gz, err := gzip.NewWriterLevel(buf, 6)
	if err != nil {
		return nil, errors.Tag(err, "init gzip")
	}
	if _, err = gz.Write(blob); err != nil {
		return nil, errors.Tag(err, "gzip")
	}
	if err = gz.Close(); err != nil {
		return nil, errors.Tag(err, "close gzip")
	}
	return buf.Bytes(), err
}
