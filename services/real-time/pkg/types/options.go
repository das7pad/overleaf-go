// Golang port of Overleaf
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
	documentUpdaterTypes "github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
)

type Options struct {
	PendingUpdatesListShardCount int64 `json:"pending_updates_list_shard_count"`

	APIs struct {
		DocumentUpdater struct {
			Options *documentUpdaterTypes.Options `json:"options"`
			URL     sharedTypes.URL               `json:"url"`
		} `json:"document_updater"`
		WebApi struct {
			Monolith bool            `json:"monolith"`
			URL      sharedTypes.URL `json:"url"`
		} `json:"web_api"`
	} `json:"apis"`
}

func (o *Options) Validate() error {
	if o.PendingUpdatesListShardCount <= 0 {
		return &errors.ValidationError{
			Msg: "pending_updates_list_shard_count must be greater than 0",
		}
	}

	if o.APIs.DocumentUpdater.Options != nil {
		if err := o.APIs.DocumentUpdater.Options.Validate(); err != nil {
			return errors.Tag(err, "apis.document_updater.options is invalid")
		}
	} else if err := o.APIs.DocumentUpdater.URL.Validate(); err != nil {
		return errors.Tag(err, "apis.document_updater.url is invalid")
	}
	if err := o.APIs.WebApi.URL.Validate(); err != nil {
		return errors.Tag(err, "apis.web_api.url is invalid")
	}
	return nil
}
