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

package fileTree

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

const (
	fileUploadsStaleAfter = 15 * time.Minute
	purgeFileUploadsAfter = fileUploadsStaleAfter + time.Minute
)

func (m *manager) CleanupStaleFileUploads(ctx context.Context, dryRun bool, start time.Time) error {
	nFailed := 0
	err := m.pm.ProcessStaleFileUploads(
		ctx,
		start.Add(-purgeFileUploadsAfter),
		func(projectId, fileId sharedTypes.UUID) bool {
			if dryRun {
				log.Printf(
					"dry-run file upload cleanup: %s/%s",
					projectId, fileId,
				)
				return false
			}
			if err := m.cleanupFileUpload(ctx, projectId, fileId); err != nil {
				err = errors.Tag(
					err,
					fmt.Sprintf(
						"file upload cleanup failed: %s/%s",
						projectId, fileId,
					),
				)
				nFailed++
				log.Println(err.Error())
			}
			return nFailed == 0
		},
	)
	if err != nil {
		err = errors.Tag(err, "query stale file uploads")
	}
	if nFailed != 0 {
		err = errors.Merge(err, errors.New(fmt.Sprintf(
			"purging failed for %d file uploads", nFailed,
		)))
	}
	return err
}

func (m *manager) cleanupFileUpload(ctx context.Context, projectId, fileId sharedTypes.UUID) error {
	if err := m.fm.DeleteProjectFile(ctx, projectId, fileId); err != nil {
		if !errors.IsNotFoundError(err) {
			return errors.Tag(err, "delete blob")
		}
	}
	if err := m.pm.PurgeStaleFileUpload(ctx, projectId, fileId); err != nil {
		return errors.Tag(err, "purge tree node")
	}
	return nil
}
