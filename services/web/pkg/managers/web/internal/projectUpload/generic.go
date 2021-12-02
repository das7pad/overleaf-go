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
	"io"

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/mongoTx"
	"github.com/das7pad/overleaf-go/pkg/objectStorage"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/fileTree"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

const (
	parallelUploads = 5
)

type uploadQueueEntry struct {
	parent  *project.Folder
	element project.TreeElement
	file    io.ReadCloser
	size    int64
	s       sharedTypes.Snapshot
}

type uploadedQueueEntry struct {
	element project.TreeElement
	parent  *project.Folder
}

func reOpenFile(file types.CreateProjectFile, f io.ReadCloser) (io.ReadCloser, error) {
	if err := f.Close(); err != nil {
		return nil, errors.Tag(err, "cannot close file")
	}
	f, err := file.Open()
	if err != nil {
		return nil, errors.Tag(err, "cannot re-open file")
	}
	return f, nil
}

func (m *manager) CreateProject(ctx context.Context, request *types.CreateProjectRequest, response *types.CreateProjectResponse) error {
	if err := request.Validate(); err != nil {
		return err
	}
	p := project.NewProject(request.UserId)
	p.Name = request.Name
	p.ImageName = m.options.DefaultImage

	foundRootDoc := false
	parentCache := make(map[sharedTypes.DirName]*project.Folder)
	t, _ := p.GetRootFolder()

	errCreate := mongoTx.For(m.db, ctx, func(sCtx context.Context) error {
		for _, f := range parentCache {
			// Reset after partial upload.
			f.Docs = f.Docs[:0]
		}
		uploadQueue := make(chan uploadQueueEntry, parallelUploads)
		uploadedQueue := make(chan uploadedQueueEntry, parallelUploads)
		eg, pCtx := errgroup.WithContext(sCtx)
		go func() {
			<-pCtx.Done()
			if pCtx.Err() != nil {
				// Purge the queue as soon as any consumer/producer fails.
				for qe := range uploadQueue {
					if qe.file != nil {
						_ = qe.file.Close()
					}
				}
			}
		}()
		eg.Go(func() error {
			defer close(uploadQueue)

			done := pCtx.Done()
			for _, file := range request.Files {
				select {
				case <-done:
					return pCtx.Err()
				default:
				}
				path := file.Path()
				size := file.Size()
				f, err := file.Open()
				if err != nil {
					return errors.Tag(err, "cannot open file")
				}
				dir := path.Dir()
				name := path.Filename()
				s, isDoc, consumedFile, err := fileTree.IsTextFile(name, size, f)
				if err != nil {
					return err
				}
				parent, exists := parentCache[dir]
				if !exists {
					parent, err = t.CreateParents(dir)
					if err != nil {
						return err
					}
					parentCache[dir] = parent
				}
				if isDoc {
					if err = f.Close(); err != nil {
						return errors.Tag(err, "cannot close file")
					}
					if e, _ := parent.GetEntry(name); e != nil {
						// already scanned
						uploadQueue <- uploadQueueEntry{
							parent:  parent,
							element: e,
							s:       s,
						}
						continue
					}
					d := project.NewDoc(name)
					uploadQueue <- uploadQueueEntry{
						parent:  parent,
						element: d,
						s:       s,
					}
					if path == "main.tex" ||
						(!foundRootDoc && path.Type().ValidForRootDoc()) {
						if isRootDoc, title := scanContent(s); isRootDoc {
							p.RootDocId = d.Id
							foundRootDoc = true
							if request.HasDefaultName && title != "" {
								p.Name = title
							}
						}
					}
				} else {
					if parent.HasEntry(name) {
						// already uploaded
						if err = f.Close(); err != nil {
							return errors.Tag(err, "cannot close file")
						}
						continue
					}
					if consumedFile {
						if f, err = reOpenFile(file, f); err != nil {
							return err
						}
					}
					var hash sharedTypes.Hash
					hash, err = fileTree.HashFile(f, size)
					if err != nil {
						return err
					}
					if f, err = reOpenFile(file, f); err != nil {
						return err
					}
					fileRef := project.NewFileRef(name, hash, size)
					uploadQueue <- uploadQueueEntry{
						parent:  parent,
						element: fileRef,
						file:    f,
						size:    size,
					}
				}
			}
			return nil
		})
		uploadEg, uploadCtx := errgroup.WithContext(pCtx)
		for i := 0; i < parallelUploads; i++ {
			uploadEg.Go(func() error {
				for queueEntry := range uploadQueue {
					switch e := queueEntry.element.(type) {
					case *project.Doc:
						err := m.dm.CreateDocWithContent(
							uploadCtx, p.Id, e.Id, queueEntry.s,
						)
						if err != nil {
							return errors.Tag(err, "cannot upload doc")
						}
					case *project.FileRef:
						err := m.fm.SendStreamForProjectFile(
							uploadCtx,
							p.Id,
							e.Id,
							queueEntry.file,
							objectStorage.SendOptions{
								ContentSize: queueEntry.size,
							},
						)
						if err != nil {
							_ = queueEntry.file.Close()
							return errors.Tag(err, "cannot upload file")
						}
						if err = queueEntry.file.Close(); err != nil {
							return errors.Tag(err, "cannot close file")
						}
					}
					uploadedQueue <- uploadedQueueEntry{
						element: queueEntry.element,
						parent:  queueEntry.parent,
					}
				}
				return nil
			})
		}
		eg.Go(func() error {
			for qe := range uploadedQueue {
				switch e := qe.element.(type) {
				case *project.Doc:
					qe.parent.Docs = append(qe.parent.Docs, e)
				case *project.FileRef:
					qe.parent.FileRefs = append(qe.parent.FileRefs, e)
				}
			}
			return nil
		})
		eg.Go(func() error {
			err := uploadEg.Wait()
			close(uploadedQueue)
			return err
		})

		var existingProjectNames project.Names
		eg.Go(func() error {
			names, err := m.pm.GetProjectNames(pCtx, request.UserId)
			if err != nil {
				return errors.Tag(err, "cannot get project names")
			}
			existingProjectNames = names
			return nil
		})

		if err := eg.Wait(); err != nil {
			return err
		}

		if len(t.Docs)+len(t.FileRefs) == 0 && len(t.Folders) == 1 {
			// Skip one level of directories and drop name of root folder.
			p.RootFolder = t.Folders
			p.RootFolder[0].Name = ""
		}

		p.Name = existingProjectNames.MakeUnique(p.Name)

		if err := m.pm.CreateProject(sCtx, p); err != nil {
			return errors.Tag(err, "cannot create project in mongo")
		}
		return nil
	})

	if errCreate == nil {
		response.Success = true
		response.ProjectId = &p.Id
		return nil
	}
	errMerged := &errors.MergedError{}
	errMerged.Add(errors.Tag(errCreate, "cannot create project"))
	errMerged.Add(m.purgeFilestoreData(p.Id))
	return errMerged
}
