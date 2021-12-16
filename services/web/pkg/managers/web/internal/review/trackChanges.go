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

package review

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/doc"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) AcceptReviewChanges(ctx context.Context, r *types.AcceptReviewChangesRequest) error {
	err := m.dum.AcceptReviewChanges(ctx, r.ProjectId, r.DocId, r.ChangeIds)
	if err != nil {
		return errors.Tag(err, "cannot accept changes")
	}
	m.notifyEditor(
		r.ProjectId, "accept-changes", r.DocId, r.ChangeIds,
	)
	return nil
}

func (m *manager) getAllRanges(ctx context.Context, projectId primitive.ObjectID) ([]doc.Ranges, error) {
	if err := m.dum.FlushProject(ctx, projectId); err != nil {
		return nil, errors.Tag(err, "cannot flush project")
	}
	ranges, err := m.dm.GetAllRanges(ctx, projectId)
	if err != nil {
		return nil, errors.Tag(err, "cannot get ranges")
	}
	return ranges, nil
}

func (m *manager) GetReviewRanges(ctx context.Context, r *types.GetReviewRangesRequest, response *types.GetReviewRangesResponse) error {
	ranges, err := m.getAllRanges(ctx, r.ProjectId)
	if err != nil {
		return err
	}
	*response = ranges
	return nil
}

func (m *manager) GetReviewUsers(ctx context.Context, r *types.GetReviewUsersRequest, response *types.GetReviewUsersResponse) error {
	ranges, err := m.getAllRanges(ctx, r.ProjectId)
	if err != nil {
		return err
	}
	userIds := make(user.UniqUserIds, len(ranges))
	for _, d := range ranges {
		for _, change := range d.Ranges.Changes {
			if id := change.MetaData.UserId; id != nil {
				userIds[*id] = true
			}
		}
	}
	users, err := m.um.GetUsersForBackFillingNonStandardId(ctx, userIds)
	if err != nil {
		return errors.Tag(err, "cannot get users")
	}
	flat := make([]*user.WithPublicInfoAndNonStandardId, 0, len(users))
	for _, u := range users {
		flat = append(flat, u)
	}
	*response = flat
	return nil
}

func (m *manager) SetTrackChangesState(ctx context.Context, r *types.SetTrackChangesStateRequest) error {
	if err := r.Validate(); err != nil {
		return err
	}
	p := &project.TrackChangesStateField{}
	if err := m.pm.GetProject(ctx, r.ProjectId, p); err != nil {
		return errors.Tag(err, "cannot get current state")
	}

	s := p.TrackChangesState
	if r.On != nil {
		s = s.SetGlobally(*r.On)
	} else {
		s = s.EnableFor(r.OnFor, r.OnForGuests)
	}

	if err := m.pm.SetTrackChangesState(ctx, r.ProjectId, s); err != nil {
		return errors.Tag(err, "cannot persist state")
	}
	go m.notifyEditor(r.ProjectId, "toggle-track-changes", s)
	return nil
}
