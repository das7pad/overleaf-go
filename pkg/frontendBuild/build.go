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
	"log"
	"time"

	"github.com/evanw/esbuild/pkg/api"
	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

func (o *outputCollector) Build(concurrency int, watch bool) error {
	t0 := time.Now()
	eg := &errgroup.Group{}
	eg.SetLimit(concurrency)

	for _, options := range getConfigs(o.root) {
		cfg := options
		var firstBuild chan struct{}
		if watch {
			firstBuild = make(chan struct{})
		}
		cfg.Plugins = append(cfg.Plugins, o.plugin(cfg, firstBuild))
		if watch && cfg.ListenForRebuild {
			cfg.Inject = append(
				cfg.Inject, join(o.root, "esbuild/inject/listenForRebuild.js"),
			)
		}
		eg.Go(func() error {
			c, ctxErr := api.Context(cfg.BuildOptions)
			if ctxErr != nil {
				return errors.Tag(ctxErr, cfg.Description)
			}
			if watch {
				if err := c.Watch(api.WatchOptions{}); err != nil {
					return errors.Tag(err, cfg.Description)
				}
				<-firstBuild
			} else {
				c.Rebuild()
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
		log.Println("static", time.Since(t1).String())
		return nil
	})

	if err := eg.Wait(); err != nil {
		return err
	}
	log.Println("build", time.Since(t0).String())
	return nil
}
