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
	"log"
	"net/http"
	"os"
	"runtime"
	"time"
)

func main() {
	addr := ":54321"
	flag.StringVar(&addr, "addr", addr, "")
	root := os.Getenv("WEB_ROOT")
	flag.StringVar(&root, "src", root, "path to node-monorepo")
	dst := "/tmp/public.tar.gz"
	flag.StringVar(&dst, "dst", dst, "")
	watch := true
	flag.BoolVar(&watch, "watch", watch, "")
	concurrency := runtime.NumCPU()
	flag.IntVar(&concurrency, "concurrency", concurrency, "")
	flag.Parse()

	t0 := time.Now()
	o := newOutputCollector(root, !watch)

	if err := o.Build(concurrency, watch); err != nil {
		panic(err)
	}

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
	log.Println("bundle", time.Since(t1).String())

	tarGz := buf.Bytes()
	sum := hash(tarGz)
	log.Printf("public.tar.gz hash=%s size=%d", sum, len(tarGz))

	if err := os.WriteFile(dst, tarGz, 0o644); err != nil {
		panic(err)
	}

	err := os.WriteFile(dst+".checksum.txt", []byte(sum), 0o644)
	if err != nil {
		panic(err)
	}
	log.Println("total", time.Since(t0).String())
}

func hash(blob []byte) string {
	d := sha256.Sum256(blob)
	return hex.EncodeToString(d[:])
}
