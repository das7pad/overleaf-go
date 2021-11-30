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

	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/doc"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/mongoTx"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type copyFileQueueEntry struct {
	parent *project.Folder
	source *project.FileRef
	target *project.FileRef
}

func (m *manager) CloneProject(ctx context.Context, request *types.CloneProjectRequest, response *types.CloneProjectResponse) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	if err := request.Name.Validate(); err != nil {
		return err
	}
	sourceProjectId := request.ProjectId
	userId := request.Session.User.Id

	lastVersion := sharedTypes.Version(-1)
	p := project.NewProject(userId)
	p.Name = request.Name
	var parentCache map[sharedTypes.DirName]*project.Folder
	var t *project.Folder

	errClone := mongoTx.For(m.db, ctx, func(sCtx context.Context) error {
		sp := &project.ForClone{}
		if err := m.pm.GetProject(ctx, sourceProjectId, sp); err != nil {
			return errors.Tag(err, "cannot get source project")
		}
		if _, err := sp.GetPrivilegeLevelAuthenticated(userId); err != nil {
			return err
		}
		st, errNoRootFolder := sp.GetRootFolder()
		if errNoRootFolder != nil {
			return errNoRootFolder
		}

		if lastVersion != sp.Version {
			if lastVersion != -1 {
				// Cleanup previous attempt.
				if err := m.purgeFilestoreData(p.Id); err != nil {
					return err
				}
			}
			lastVersion = sp.Version
			parentCache = make(map[sharedTypes.DirName]*project.Folder)
			t = project.NewFolder("")
			p.RootFolder[0] = t
			p.RootDocId = primitive.NilObjectID
		}
		p.ImageName = sp.ImageName
		p.Compiler = sp.Compiler
		p.SpellCheckLanguage = sp.SpellCheckLanguage

		errCreateFolders := st.WalkFolders(func(src *project.Folder, dir sharedTypes.DirName) error {
			if dst, exists := parentCache[dir]; exists {
				// Already created in previous tx cycle. Clear docs.
				dst.Docs = dst.Docs[:0]
				return nil
			}
			if err := src.CheckHasUniqueEntries(); err != nil {
				return err
			}
			dst, err := t.CreateParents(dir)
			if err != nil {
				return err
			}
			parentCache[dir] = dst

			// Pre-Allocate memory
			if n := len(src.Docs); cap(dst.Docs) < n {
				dst.Docs = make([]*project.Doc, 0, n)
			}
			if n := len(src.FileRefs); cap(dst.FileRefs) < n {
				dst.FileRefs = make([]*project.FileRef, 0, n)
			}
			if n := len(src.Folders); cap(dst.Folders) < n {
				dst.Folders = make([]*project.Folder, 0, n)
			}
			return nil
		})
		if errCreateFolders != nil {
			return errCreateFolders
		}
		parentCache["."] = t

		copyFileQueue := make(chan *copyFileQueueEntry, parallelUploads)
		doneCopyingFileQueue := make(chan *copyFileQueueEntry, parallelUploads)
		eg, pCtx := errgroup.WithContext(sCtx)
		go func() {
			<-pCtx.Done()
			if pCtx.Err() != nil {
				// Purge the queue as soon as any consumer/producer fails.
				for range copyFileQueue {
				}
			}
		}()
		eg.Go(func() error {
			if err := m.dum.FlushProject(ctx, sourceProjectId); err != nil {
				return errors.Tag(err, "cannot flush docs to mongo")
			}
			docs, err := m.dm.GetAllDocContents(ctx, sourceProjectId)
			if err != nil {
				return errors.Tag(err, "cannot get docs")
			}
			docPaths := make(map[primitive.ObjectID]sharedTypes.PathName, len(docs))

			err = st.WalkDocs(func(element project.TreeElement, path sharedTypes.PathName) error {
				docPaths[element.GetId()] = path
				return nil
			})
			if err != nil {
				return err
			}
			if len(docs) != len(docPaths) {
				return &errors.InvalidStateError{
					Msg: "projects and docs collection do not agree on docs",
				}
			}

			newDocs := make([]doc.Contents, len(docs))
			for i, contents := range docs {
				path, exists := docPaths[contents.Id]
				if !exists {
					return &errors.InvalidStateError{
						Msg: "missing doc in project tree",
					}
				}
				d := project.NewDoc(path.Filename())
				if contents.Id == sp.RootDocId &&
					path.Type().ValidForRootDoc() {
					isRootDocCandidate, _ := scanContent(
						contents.Lines.ToSnapshot(),
					)
					if isRootDocCandidate {
						p.RootDocId = d.Id
					}
				}
				contents.Id = d.Id
				newDocs[i] = contents
				parent := parentCache[path.Dir()]
				parent.Docs = append(parent.Docs, d)
			}
			err = m.dm.CreateDocsWithContent(pCtx, p.Id, newDocs)
			if err != nil {
				return errors.Tag(err, "cannot create docs")
			}
			return nil
		})
		eg.Go(func() error {
			defer close(copyFileQueue)

			err := st.WalkFiles(func(element project.TreeElement, path sharedTypes.PathName) error {
				sourceFileRef := element.(*project.FileRef)
				dir := path.Dir()
				name := sourceFileRef.Name
				parent := parentCache[dir]
				for _, f := range parent.FileRefs {
					if f.Name == name {
						// already uploaded
						return nil
					}
				}
				fileRef := project.NewFileRef(
					name, sourceFileRef.Hash, sourceFileRef.Size,
				)
				fileRef.LinkedFileData = sourceFileRef.LinkedFileData
				copyFileQueue <- &copyFileQueueEntry{
					parent: parent,
					source: sourceFileRef,
					target: fileRef,
				}
				return nil
			})
			return err
		})
		uploadEg, uploadCtx := errgroup.WithContext(pCtx)
		for i := 0; i < parallelUploads; i++ {
			uploadEg.Go(func() error {
				for queueEntry := range copyFileQueue {
					err := m.fm.CopyProjectFile(
						uploadCtx,
						sourceProjectId,
						queueEntry.source.Id,
						p.Id,
						queueEntry.target.Id,
					)
					if err != nil {
						return errors.Tag(err, "cannot copy file")
					}
					doneCopyingFileQueue <- queueEntry
				}
				return nil
			})
		}
		eg.Go(func() error {
			for qe := range doneCopyingFileQueue {
				qe.parent.FileRefs = append(qe.parent.FileRefs, qe.target)
			}
			return nil
		})
		eg.Go(func() error {
			err := uploadEg.Wait()
			close(doneCopyingFileQueue)
			return err
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

		if err := eg.Wait(); err != nil {
			return err
		}

		p.Name = existingProjectNames.MakeUnique(p.Name)

		if err := m.pm.CreateProject(sCtx, p); err != nil {
			return errors.Tag(err, "cannot create project in mongo")
		}
		return nil
	})

	if errClone == nil {
		response.ProjectId = &p.Id
		response.Name = p.Name
		response.Owner = request.Session.User.ToPublicUserInfo()
		return nil
	}
	errMerged := &errors.MergedError{}
	errMerged.Add(errors.Tag(errClone, "cannot clone project"))
	errMerged.Add(m.purgeFilestoreData(p.Id))
	return errMerged
}
