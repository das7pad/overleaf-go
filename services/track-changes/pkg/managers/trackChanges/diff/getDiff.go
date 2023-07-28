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

package diff

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/sharejs/types/text"
	"github.com/das7pad/overleaf-go/services/track-changes/pkg/types"
)

type diffRopeEntry struct {
	types.DiffEntry
	next *diffRopeEntry
}

func (q *diffRopeEntry) Split(offset int) {
	if len(q.Insertion) > 0 {
		q.next = &diffRopeEntry{
			DiffEntry: types.DiffEntry{
				Meta:      q.Meta,
				Insertion: q.Insertion[offset:],
			},
			next: q.next,
		}
		q.Insertion = q.Insertion[:offset]
	} else {
		q.next = &diffRopeEntry{
			DiffEntry: types.DiffEntry{
				Meta:      q.Meta,
				Unchanged: q.Unchanged[offset:],
			},
			next: q.next,
		}
		q.Unchanged = q.Unchanged[:offset]
	}
}

func (m *manager) GetDocDiff(ctx context.Context, r *types.GetDocDiffRequest, response *types.GetDocDiffResponse) error {
	if err := r.Validate(); err != nil {
		return err
	}
	projectId := r.ProjectId
	docId := r.DocId

	// get the base-line plus history entries
	s, dh, err := m.getDocFrom(ctx, projectId, r.UserId, docId, r.From, r.To)
	if err != nil {
		return err
	}

	// build a rope of diffs
	head := diffRopeEntry{
		next: &diffRopeEntry{
			DiffEntry: types.DiffEntry{
				Unchanged: sharedTypes.Snippet(s),
				Meta: types.DiffMeta{
					User:    dh.Users.GetUserNonStandardId(sharedTypes.UUID{}),
					StartTS: 0,
					EndTS:   0,
				},
			},
		},
	}
	n := 1
	for _, history := range dh.History {
		meta := types.DiffMeta{
			User:    dh.Users.GetUserNonStandardId(history.UserId),
			StartTS: sharedTypes.Timestamp(history.StartAt.UnixMilli()),
			EndTS:   sharedTypes.Timestamp(history.EndAt.UnixMilli()),
		}
		for _, component := range history.Op {
			p := 0
			componentPosition := component.Position
			after := &head
			for after.next != nil {
				q := after.next
				if len(q.Deletion) > 0 {
					after = q
					continue
				}
				if p == componentPosition {
					break
				}
				end := p + len(q.Insertion) + len(q.Unchanged)
				if end == componentPosition {
					after = q
					break
				}
				if end > componentPosition {
					n++
					q.Split(componentPosition - p)
					after = q
					break
				}
				p = end
				after = q
			}

			if len(component.Deletion) == 0 {
				n++
				after.next = &diffRopeEntry{
					DiffEntry: types.DiffEntry{
						Insertion: component.Insertion,
						Meta:      meta,
					},
					next: after.next,
				}
				continue
			}
			// consume deletion
			for after.next != nil && len(component.Deletion) > 0 {
				q := after.next
				if len(q.Deletion) > 0 {
					after = q
					continue
				}
				l := len(q.Insertion) + len(q.Unchanged)
				if len(component.Deletion) < l {
					// delete part of q
					n++
					q.Split(len(component.Deletion))
					continue
				}
				// delete all of q
				if len(q.Unchanged) > 0 {
					// deletion on green field
					q.Deletion = q.Unchanged
					q.Unchanged = nil
					q.Meta = meta
				} else {
					// hide a previous insertion
					n--
					after.next = q.next
				}
				component.Deletion = component.Deletion[l:]
			}
		}
	}

	// merge
	for q := head.next; q.next != nil; q = q.next {
		if q.Meta.User.Id != q.next.Meta.User.Id {
			continue
		}
		switch {
		case len(q.Insertion) > 0 && len(q.next.Insertion) > 0:
			// The underlying snippet may be re-used from splitting the entry.
			// We need to copy it before appending.
			q.Insertion = text.Inject(
				q.Insertion, len(q.Insertion), q.next.Insertion,
			)
		case len(q.Deletion) > 0 && len(q.next.Deletion) > 0:
			q.Deletion = text.Inject(
				q.Deletion, len(q.Deletion), q.next.Deletion,
			)
		default:
			continue
		}
		if q.next.Meta.StartTS < q.Meta.StartTS {
			q.Meta.StartTS = q.next.Meta.StartTS
		}
		if q.next.Meta.EndTS > q.Meta.EndTS {
			q.Meta.EndTS = q.next.Meta.EndTS
		}
		n--
		q.next = q.next.next
	}

	response.Diff = make([]types.DiffEntry, 0, n)
	for q := head.next; q != nil; q = q.next {
		response.Diff = append(response.Diff, q.DiffEntry)
	}
	return nil
}
