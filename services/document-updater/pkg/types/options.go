// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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

package types

import (
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/options/env"
	"github.com/das7pad/overleaf-go/pkg/redisScanner"
)

type Options struct {
	PeriodicFlushAll             redisScanner.PeriodicOptions `json:"periodic_flush_all"`
	Workers                      int                          `json:"workers"`
	PendingUpdatesListShardCount int                          `json:"pending_updates_list_shard_count"`
}

func (o *Options) FillFromEnv() {
	env.MustParseJSON(o, "DOCUMENT_UPDATER_OPTIONS")
}

func (o *Options) Validate() error {
	if o.PendingUpdatesListShardCount <= 0 {
		return &errors.ValidationError{
			Msg: "pending_updates_list_shard_count must be greater than 0",
		}
	}
	if o.Workers <= 0 {
		return &errors.ValidationError{
			Msg: "workers must be greater than 0",
		}
	}
	if err := o.PeriodicFlushAll.Validate(); err != nil {
		return errors.Tag(err, "periodic_flush_all")
	}
	return nil
}
