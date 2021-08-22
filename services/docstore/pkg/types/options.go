// Golang port of the Overleaf docstore service
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
	"github.com/das7pad/overleaf-go/services/docstore/pkg/backend"
)

type Options struct {
	ArchivePLimits PLimits         `json:"archive_p_limits"`
	BackendOptions backend.Options `json:"backend_options"`
	Bucket         string          `json:"bucket"`
	MaxDeletedDocs Limit           `json:"max_deleted_docs"`
}

func (o Options) Validate() error {
	if o.Bucket == "" {
		return &errors.ValidationError{
			Msg: "missing bucket",
		}
	}
	if o.MaxDeletedDocs <= 0 {
		return &errors.ValidationError{
			Msg: "max_deleted_docs must be greater 0",
		}
	}
	if o.ArchivePLimits.BatchSize <= 0 {
		return &errors.ValidationError{
			Msg: "archive_p_limits.batchSize must be greater 0",
		}
	}
	if o.ArchivePLimits.ParallelArchiveJobs <= 0 {
		return &errors.ValidationError{
			Msg: "archive_p_limits.parallelArchiveJobs must be greater 0",
		}
	}
	return nil
}
