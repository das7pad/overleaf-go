// Golang port of Overleaf
// Copyright (C) 2022 Jakob Ackermann <das7pad@outlook.com>
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
	"testing"

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/spelling/pkg/types"
)

func BenchmarkWorkerPool_CheckWords(b *testing.B) {
	wp := newWorkerPool()
	defer wp.Close()
	eg, ctx := errgroup.WithContext(context.Background())

	for j := 0; j < MaxWorkers; j++ {
		lng := types.SpellCheckLanguage("en")
		switch {
		case j%2 == 0:
			lng = "en"
		case j%3 == 0:
			lng = "bg"
		case j%5 == 0:
			lng = "de"
		case j%7 == 0:
			lng = "es"
		case j%11 == 0:
			lng = "fr"
		case j%13 == 0:
			lng = "pt_BR"
		case j%17 == 0:
			lng = "pt_PT"
		}
		eg.Go(func() error {
			for i := 0; i < b.N; i++ {
				_, err := wp.CheckWords(ctx, lng, []string{"Overleaf"})
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
	wp.(*workerPool).l.Lock()
	b.Log(wp.(*workerPool).freeSlots)
	b.Log(wp.(*workerPool).slots)
	wp.(*workerPool).l.Unlock()
}
