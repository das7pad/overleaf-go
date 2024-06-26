// Golang port of Overleaf
// Copyright (C) 2022-2024 Jakob Ackermann <das7pad@outlook.com>
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

package flush

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func getProjectTrackingKey(projectId sharedTypes.UUID) string {
	b := make([]byte, 0, 20+36+1)
	b = append(b, "DocsWithHistoryOps:{"...)
	b = projectId.Append(b)
	b = append(b, '}')
	return string(b)
}

func (m *manager) FlushProject(ctx context.Context, projectId sharedTypes.UUID) error {
	projectTracking := getProjectTrackingKey(projectId)
	docIdsRaw, errList := m.client.SMembers(ctx, projectTracking).Result()
	if errList != nil {
		return errors.Tag(errList, "list docs in project")
	}
	if len(docIdsRaw) == 0 {
		return nil
	}

	queue := make(chan sharedTypes.UUID, len(docIdsRaw))
	defer func() {
		for range queue {
		}
	}()
	for _, s := range docIdsRaw {
		id, err := sharedTypes.ParseUUID(s)
		if err != nil {
			close(queue)
			return errors.Tag(
				err, fmt.Sprintf("parse %s as doc id", s),
			)
		}
		queue <- id
	}
	close(queue)
	eg, pCtx := errgroup.WithContext(ctx)
	for i := 0; i < 5; i++ {
		eg.Go(func() error {
			for docId := range queue {
				err := m.FlushDoc(pCtx, projectId, docId)
				if err != nil {
					return errors.Tag(
						err,
						fmt.Sprintf("process updates for doc %s", docId),
					)
				}
			}
			return nil
		})
	}
	return eg.Wait()
}
