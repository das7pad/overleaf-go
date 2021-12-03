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

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/doc"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/mongoTx"
	"github.com/das7pad/overleaf-go/pkg/objectStorage"
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

	p := project.NewProject(userId)
	p.ImageName = m.options.DefaultImage
	p.Name = request.Name
	t, _ := p.GetRootFolder()

	viewData := &exampleProjects.ViewData{
		ProjectCreationDate: time.Now().UTC(),
		ProjectName:         request.Name,
		Owner:               request.Session.User.ToPublicUserInfo(),
	}

	errCreate := mongoTx.For(m.db, ctx, func(sCtx context.Context) error {
		docs, files, errRender := exampleProject.Render(viewData)
		if errRender != nil {
			return errRender
		}

		_ = t.WalkFolders(func(f *project.Folder, _ sharedTypes.DirName) error {
			// Reset after partial upload.
			f.Docs = f.Docs[:0]
			return nil
		})

		eg, pCtx := errgroup.WithContext(sCtx)
		eg.Go(func() error {
			newDocs := make([]doc.Contents, len(docs))
			for i, content := range docs {
				d := project.NewDoc(content.Path.Filename())
				if content.Path == "main.tex" {
					p.RootDocId = d.Id
				}
				newDocs[i].Id = d.Id
				newDocs[i].Lines = content.Snapshot.ToLines()
				parent, err := t.CreateParents(content.Path.Dir())
				if err != nil {
					return err
				}
				parent.Docs = append(parent.Docs, d)
			}
			err := m.dm.CreateDocsWithContent(pCtx, p.Id, newDocs)
			if err != nil {
				return errors.Tag(err, "cannot create docs")
			}
			return nil
		})

		eg.Go(func() error {
			for _, file := range files {
				parent, errConflict := t.CreateParents(file.Path.Dir())
				if errConflict != nil {
					return errConflict
				}
				if parent.HasEntry(file.Path.Filename()) {
					// already uploaded
					continue
				}
				fileRef := project.NewFileRef(
					file.Path.Filename(), file.Hash, file.Size,
				)
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
				parent.FileRefs = append(parent.FileRefs, fileRef)
			}
			return nil
		})

		var existingProjectNames project.Names
		eg.Go(func() error {
			names, err := m.pm.GetProjectNames(pCtx, userId)
			if err != nil {
				return errors.Tag(err, "cannot get project names")
			}
			existingProjectNames = names
			return nil
		})
		eg.Go(func() error {
			return m.setSpellCheckLanguage(
				ctx, &p.SpellCheckLanguageField, userId,
			)
		})

		if err := eg.Wait(); err != nil {
			return err
		}

		p.Name = existingProjectNames.MakeUnique(p.Name)

		if err := m.pm.CreateProject(sCtx, p); err != nil {
			return errors.Tag(err, "cannot create project in mongo")
		}
		return nil
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
