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

package diff

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/docHistory"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/sharejs/types/text"
	"github.com/das7pad/overleaf-go/services/track-changes/pkg/managers/trackChanges/flush"
	"github.com/das7pad/overleaf-go/services/track-changes/pkg/types"
)

type Manager interface {
	GetDocDiff(ctx context.Context, request *types.GetDocDiffRequest, response *types.GetDocDiffResponse) error
	RestoreDocVersion(ctx context.Context, request *types.RestoreDocVersionRequest) error
}

func New(dhm docHistory.Manager, fm flush.Manager, dum documentUpdater.Manager) Manager {
	return &manager{
		dhm: dhm,
		dum: dum,
		fm:  fm,
	}
}

type manager struct {
	dhm docHistory.Manager
	dum documentUpdater.Manager
	fm  flush.Manager
}

func (m *manager) getDocFrom(ctx context.Context, projectId, docId sharedTypes.UUID, from, to sharedTypes.Version) (sharedTypes.Snapshot, *docHistory.GetForDocResult, error) {
	d, err := m.dum.GetDoc(ctx, projectId, docId, -1)
	if err != nil {
		return nil, nil, errors.Tag(err, "cannot get latest doc version")
	}
	// NOTE: The flush could get replaced with a more complex catch-up process:
	//       Assume that only the editor UI requests diffs, and it flushes
	//        ahead of displaying a list of docs+version-ranges that were in
	//        turn taken from the db/flushed state.
	//       So we can use the redis state for rewinding the doc and use the
	//        db state for building diffs.
	//       With retries: get latest doc + pending history queue and catch up
	if err = m.fm.FlushDoc(ctx, projectId, docId); err != nil {
		return nil, nil, errors.Tag(err, "cannot flush doc history")
	}
	dh := docHistory.GetForDocResult{
		History: make([]docHistory.DocHistory, 0, 1+d.Version-from),
		Users:   make(user.BulkFetched, 10),
	}
	err = m.dhm.GetForDoc(ctx, projectId, docId, from, d.Version, &dh)
	if err != nil {
		return nil, nil, errors.Tag(err, "cannot get flushed history")
	}
	s := sharedTypes.Snapshot(d.Snapshot)
	dropFrom := len(dh.History)

	// rewind the doc
	rev := make(sharedTypes.Op, 10)
	for i := len(dh.History) - 1; i >= 0; i-- {
		op := dh.History[i].Op
		n := len(op)
		if n > cap(rev) {
			rev = make(sharedTypes.Op, n)
		} else {
			rev = rev[:n]
		}
		for j := 0; j <= n/2+n%2 && j < n; j++ {
			rev[n-1-j].Position = op[j].Position
			rev[n-1-j].Deletion, rev[n-1-j].Insertion =
				op[j].Insertion, op[j].Deletion
		}
		if s, err = text.Apply(s, rev); err != nil {
			return nil, nil, errors.Tag(err, "broken history")
		}
		if dh.History[i].Version > to {
			dropFrom = i
		}
	}

	// cut off history entries used for rewinding the doc
	dh.History = dh.History[:dropFrom]
	return s, &dh, nil
}
