// Golang port of the Overleaf document-updater service
// Copyright (C) 2021 Jakob Ackermann <das7pad@outlook.com>
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
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	docstoreTypes "github.com/das7pad/overleaf-go/services/docstore/pkg/types"
)

type Options struct {
	WorkersPerShard              int `json:"workers_per_shard"`
	PendingUpdatesListShardCount int `json:"pending_updates_list_shard_count"`

	APIs struct {
		Docstore struct {
			Options *docstoreTypes.Options `json:"options"`
		} `json:"docstore"`
		TrackChanges struct {
			URL sharedTypes.URL `json:"url"`
		} `json:"track_changes"`
		WebApi struct {
			URL      sharedTypes.URL `json:"url"`
			Monolith bool            `json:"monolith"`
		} `json:"web_api"`
	} `json:"apis"`
}

func (o *Options) Validate() error {
	if o.PendingUpdatesListShardCount <= 0 {
		return &errors.ValidationError{
			Msg: "pending_updates_list_shard_count must be greater than 0",
		}
	}
	if o.WorkersPerShard <= 0 {
		return &errors.ValidationError{
			Msg: "workers_per_shard must be greater than 0",
		}
	}

	if err := o.APIs.TrackChanges.URL.Validate(); err != nil {
		return errors.Tag(err, "track_changes.url is invalid")
	}
	if o.APIs.WebApi.Monolith {
		if err := o.APIs.Docstore.Options.Validate(); err != nil {
			return errors.Tag(err, "docstore.options is invalid")
		}
	} else {
		if err := o.APIs.WebApi.URL.Validate(); err != nil {
			return errors.Tag(err, "web_api.url is invalid")
		}
	}
	return nil
}
