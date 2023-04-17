// Golang port of Overleaf
// Copyright (C) 2022-2023 Jakob Ackermann <das7pad@outlook.com>
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

package aspellRunner

import (
	"context"
	"fmt"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/spelling/pkg/types"
)

func benchmarkWorkerPoolCheckWords(b *testing.B, p int) {
	b.ReportAllocs()
	wp := NewWorkerPool()
	defer wp.Close()
	eg, ctx := errgroup.WithContext(context.Background())

	t0 := time.Now()
	for j := 0; j < p; j++ {
		eg.Go(func() error {
			for i := 0; i < b.N; i++ {
				lng := types.SpellCheckLanguage("en")
				switch {
				case i%2 == 0:
					lng = "en"
				case i%3 == 0:
					lng = "bg"
				case i%5 == 0:
					lng = "de"
				case i%7 == 0:
					lng = "es"
				case i%11 == 0:
					lng = "fr"
				case i%13 == 0:
					lng = "pt_BR"
				case i%17 == 0:
					lng = "pt_PT"
				}
				_, err := wp.CheckWords(ctx, lng, []string{"Helllo"})
				if err != nil {
					return errors.Tag(err, string(lng))
				}
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		b.Fatal(err)
	}
	perReq := time.Since(t0) / time.Duration(p*b.N)
	b.ReportMetric(float64(perReq), "ns/req")
	b.ReportMetric(float64(time.Second/perReq), "req/s")
}

func BenchmarkWorkerPool_CheckWords(b *testing.B) {
	cases := []int{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 20, 30, MaxWorkers, MaxWorkers * 2, 100,
	}
	for i := range cases {
		p := cases[i]
		b.Run(fmt.Sprintf("%03d", p), func(b *testing.B) {
			benchmarkWorkerPoolCheckWords(b, p)
		})
	}
}
