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

package projectUpload

import (
	"context"
	"time"

	"github.com/edgedb/edgedb-go"
	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/objectStorage"
	"github.com/das7pad/overleaf-go/pkg/pendingOperation"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/projectUpload/exampleProjects"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) CreateExampleProject(ctx context.Context, request *types.CreateExampleProjectRequest, response *types.CreateExampleProjectResponse) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	if err := request.Name.Validate(); err != nil {
		return err
	}
	userId := request.Session.User.Id

	exampleProject, errUnknownTemplate := exampleProjects.Get(request.Template)
	if errUnknownTemplate != nil {
		return errUnknownTemplate
	}
	viewData := &exampleProjects.ViewData{
		ProjectCreationDate: time.Now().UTC(),
		ProjectName:         request.Name,
		Owner:               request.Session.User.ToPublicUserInfo(),
	}

	var p *project.ForCreation
	errCreate := m.c.Tx(ctx, func(ctx context.Context, tx *edgedb.Tx) error {
		p = project.NewProject(userId)
		p.ImageName = m.options.DefaultImage
		t := p.RootFolder
		makeUniqName := pendingOperation.TrackOperationWithCancel(
			ctx,
			func(ctx context.Context) error {
				names, err := m.pm.GetProjectNames(ctx, userId)
				if err != nil {
					return errors.Tag(err, "cannot get project names")
				}
				p.Name = names.MakeUnique(request.Name)
				return nil
			},
		)
		defer makeUniqName.Cancel()

		docs, files, errRender := exampleProject.Render(viewData)
		if errRender != nil {
			return errRender
		}
		fileLookup := make(
			map[sharedTypes.PathName]*project.FileRef, len(files),
		)
		for _, content := range docs {
			d := project.NewDoc(content.Path.Filename())
			if content.Path == "main.tex" {
				p.RootDoc = project.RootDoc{Doc: d}
			}
			d.Snapshot = string(content.Snapshot)
			parent, err := t.CreateParents(content.Path.Dir())
			if err != nil {
				return err
			}
			parent.Docs = append(parent.Docs, d)
		}
		for _, file := range files {
			parent, err := t.CreateParents(file.Path.Dir())
			if err != nil {
				return err
			}
			fileRef := project.NewFileRef(
				file.Path.Filename(), file.Hash, file.Size,
			)
			parent.FileRefs = append(parent.FileRefs, fileRef)
			fileLookup[file.Path] = &parent.FileRefs[len(parent.FileRefs)-1]
		}

		if err := m.pm.PrepareProjectCreation(ctx, p); err != nil {
			return errors.Tag(err, "cannot insert project")
		}
		if err := m.pm.CreateProjectTree(ctx, p); err != nil {
			return errors.Tag(err, "cannot create folders/docs")
		}

		eg, pCtx := errgroup.WithContext(ctx)
		eg.Go(func() error {
			for _, file := range files {
				fileRef := fileLookup[file.Path]
				err := m.fm.SendStreamForProjectFile(
					pCtx,
					p.Id,
					fileRef.Id,
					file.Reader,
					objectStorage.SendOptions{
						ContentSize: file.Size,
					},
				)
				if err != nil {
					return errors.Tag(err, "cannot copy file")
				}
			}
			return nil
		})
		eg.Go(func() error {
			if err := makeUniqName.Wait(pCtx); err != nil {
				return err
			}
			err := m.pm.FinalizeProjectCreation(
				pCtx,
				p,
			)
			if err != nil {
				return errors.Tag(err, "cannot finalize project creation")
			}
			return nil
		})
		return eg.Wait()
	})

	if errCreate == nil {
		response.ProjectId = &p.Id
		response.Name = p.Name
		response.Owner = request.Session.User.ToPublicUserInfo()
		return nil
	}
	errMerged := &errors.MergedError{}
	errMerged.Add(errors.Tag(errCreate, "cannot create project"))
	errMerged.Add(m.purgeFilestoreData(p.Id))
	return errMerged
}
