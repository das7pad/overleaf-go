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
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path"
	"runtime"
	"time"

	"github.com/evanw/esbuild/pkg/api"
	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

func main() {
	addr := ":54321"
	flag.StringVar(&addr, "addr", addr, "")
	p := os.Getenv("WEB_ROOT")
	flag.StringVar(&p, "src", p, "path to node-monorepo")
	dst := "/tmp/public.tar.gz"
	flag.StringVar(&dst, "dst", dst, "")
	watch := true
	flag.BoolVar(&watch, "watch", watch, "")
	concurrency := runtime.NumCPU()
	flag.IntVar(&concurrency, "concurrency", concurrency, "")
	flag.Parse()

	t0 := time.Now()
	o := newOutputCollector(p, !watch)

	eg := &errgroup.Group{}
	eg.SetLimit(concurrency)

	for _, options := range getConfigs(p) {
		cfg := options
		cfg.Plugins = append(cfg.Plugins, o.Plugin(cfg))
		if watch && cfg.ListenForRebuild {
			cfg.Inject = append(
				cfg.Inject, path.Join(p, "esbuild/inject/listenForRebuild.js"),
			)
		}
		eg.Go(func() error {
			c, ctxErr := api.Context(cfg.BuildOptions)
			if ctxErr != nil {
				return errors.Tag(ctxErr, cfg.Description)
			}
			t1 := time.Now()
			c.Rebuild()
			fmt.Println(cfg.Description, time.Since(t1).String())
			if watch {
				if err := c.Watch(api.WatchOptions{}); err != nil {
					return errors.Tag(err, cfg.Description)
				}
			} else {
				c.Dispose()
			}
			return nil
		})
	}
	eg.Go(func() error {
		t1 := time.Now()
		if err := o.writeStaticFiles(); err != nil {
			return err
		}
		fmt.Println("static", time.Since(t1).String())
		return nil
	})

	defer func() {
		fmt.Println("total", time.Since(t0).String())
	}()

	if err := eg.Wait(); err != nil {
		panic(err)
	}
	fmt.Println("build", time.Since(t0).String())

	if watch {
		if err := http.ListenAndServe(addr, o); err != nil {
			panic(err)
		}
		return
	}

	t1 := time.Now()
	buf := bytes.NewBuffer(make([]byte, 0, 30_000_000))
	if err := o.Bundle(buf); err != nil {
		panic(err)
	}
	fmt.Println("bundle", time.Since(t1).String())

	tarGz := buf.Bytes()
	sum := hash(tarGz)
	fmt.Println(sum, len(tarGz))

	err := os.WriteFile(dst, tarGz, 0o644)
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(dst+".checksum.txt", []byte(sum), 0o644)
	if err != nil {
		panic(err)
	}
}

func hash(blob []byte) string {
	d := sha256.Sum256(blob)
	return hex.EncodeToString(d[:])
}
