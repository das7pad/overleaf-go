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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"runtime"
	"time"

	"github.com/evanw/esbuild/pkg/api"
	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

func main() {
	p := os.Getenv("WEB_ROOT")
	dst := "/tmp"
	watch := false
	bundle := false
	concurrency := runtime.NumCPU()

	buf := bytes.NewBuffer(make([]byte, 0, 30_000_000))
	o := newOutputCollector(p, bundle)

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
				t0 := time.Now()
				c.Rebuild()
				fmt.Println(cfg.Description, time.Since(t0).String())
				c.Dispose()
			}
			return nil
		})
	}
	eg.Go(func() error {
		t0 := time.Now()
		if err := writeStaticFiles(p, o); err != nil {
			return err
		}
		fmt.Println("static", time.Since(t0).String())
		return nil
	})

	t0 := time.Now()
	defer func() {
		fmt.Println("total", time.Since(t0).String())
	}()

	t := time.Now()
	if err := eg.Wait(); err != nil {
		panic(err)
	}
	fmt.Println("build", time.Since(t).String())

	if !bundle {
		return
	}

	t = time.Now()
	if err := o.Bundle(buf); err != nil {
		panic(err)
	}
	fmt.Println("bundle", time.Since(t).String())

	tarGz := buf.Bytes()
	sum := []byte(hash(tarGz))
	fmt.Println(string(sum), len(tarGz))

	err := os.WriteFile(path.Join(dst, "public.tar.gz"), tarGz, 0o644)
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(path.Join(dst, "public.tar.gz.checksum.txt"), sum, 0o644)
	if err != nil {
		panic(err)
	}
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

func hash(blob []byte) string {
	d := sha256.Sum256(blob)
	return hex.EncodeToString(d[:])
}
