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

	"github.com/edgedb/edgedb-go"
	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/objectStorage"
	"github.com/das7pad/overleaf-go/pkg/pendingOperation"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/fileTree"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

const (
	parallelUploads = 5
)

func seekToStart(file types.CreateProjectFile, f io.ReadCloser) (io.ReadCloser, error) {
	if seeker, ok := f.(io.Seeker); ok {
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			return f, errors.Tag(err, "cannot seek to start")
		}
		return f, nil
	}
	if err := f.Close(); err != nil {
		return f, errors.Tag(err, "cannot close file")
	}
	newF, err := file.Open()
	if err != nil {
		return f, errors.Tag(err, "cannot re-open file")
	}
	return newF, nil
}

func (m *manager) CreateProject(ctx context.Context, request *types.CreateProjectRequest, response *types.CreateProjectResponse) error {
	if err := request.Validate(); err != nil {
		return err
	}
	p := project.NewProject(request.UserId)
	p.Name = request.Name
	p.ImageName = m.options.DefaultImage
	if request.Compiler != "" {
		p.Compiler = request.Compiler
	}

	var existingProjectNames project.Names
	getProjectNames := pendingOperation.TrackOperationWithCancel(
		ctx,
		func(ctx context.Context) error {
			names, err := m.pm.GetProjectNames(ctx, request.UserId)
			if err != nil {
				return errors.Tag(err, "cannot get project names")
			}
			existingProjectNames = names
			return nil
		},
	)
	defer getProjectNames.Cancel()

	openReader := make(map[sharedTypes.PathName]io.ReadCloser)
	defer func() {
		for _, q := range openReader {
			_ = q.Close()
		}
	}()
	t := &p.RootFolder
	var rootDocPath sharedTypes.PathName

	errCreate := m.c.Tx(ctx, func(ctx context.Context, _ *edgedb.Tx) error {
		if p.Id != (edgedb.UUID{}) {
			if err := m.purgeFilestoreData(p.Id); err != nil {
				return err
			}
		}
		eg, pCtx := errgroup.WithContext(ctx)
		eg.Go(func() error {
			if err := m.pm.PrepareProjectCreation(ctx, p); err != nil {
				return errors.Tag(err, "cannot init project")
			}
			return nil
		})
		eg.Go(func() error {
			done := pCtx.Done()
			for _, file := range request.Files {
				select {
				case <-done:
					return pCtx.Err()
				default:
				}
				path := file.Path()
				dir := path.Dir()
				name := path.Filename()
				parent, errConflict := t.CreateParents(dir)
				if errConflict != nil {
					return errConflict
				}
				if fileRef := parent.GetFile(name); fileRef != nil {
					if _, exists := openReader[path]; !exists {
						// The previous tx iteration started an upload, retry.
						f, err := file.Open()
						if err != nil {
							return errors.Tag(err, "cannot open file")
						}
						openReader[path] = f
					}
					continue
				}
				if d := parent.GetDoc(name); d != nil {
					continue
				}
				size := file.Size()
				f, err := file.Open()
				if err != nil {
					return errors.Tag(err, "cannot open file")
				}
				s, isDoc, consumedFile, err := fileTree.IsTextFile(name, size, f)
				if err != nil {
					_ = f.Close()
					return err
				}
				if isDoc {
					if err = f.Close(); err != nil {
						return errors.Tag(err, "cannot close file")
					}
					d := project.NewDoc(name)
					if path == "main.tex" ||
						(rootDocPath == "" && path.Type().ValidForRootDoc()) {
						if isRootDoc, title := scanContent(s); isRootDoc {
							rootDocPath = path
							if request.HasDefaultName && title != "" {
								p.Name = title
							}
							if request.AddHeader != nil {
								s = request.AddHeader(s)
							}
						}
					}
					d.Snapshot = string(s)
					d.Size = int64(len(s))
					parent.Docs = append(parent.Docs, d)
				} else {
					if consumedFile {
						if f, err = seekToStart(file, f); err != nil {
							_ = f.Close()
							return err
						}
					}
					var hash sharedTypes.Hash
					if hash = file.PreComputedHash(); hash == "" {
						hash, err = fileTree.HashFile(f, size)
						if err != nil {
							_ = f.Close()
							return err
						}
						if f, err = seekToStart(file, f); err != nil {
							_ = f.Close()
							return err
						}
					}
					fileRef := project.NewFileRef(name, hash, size)
					parent.FileRefs = append(parent.FileRefs, fileRef)
					openReader[path] = f
				}
			}
			return nil
		})
		if err := eg.Wait(); err != nil {
			return err
		}

		if err := m.pm.CreateProjectTree(ctx, p); err != nil {
			return err
		}
		if rootDocPath != "" {
			parent, _ := t.CreateParents(rootDocPath.Dir())
			p.RootDoc.Doc.Id = parent.GetDoc(rootDocPath.Filename()).Id
		}

		eg, pCtx = errgroup.WithContext(ctx)
		uploadQueue := make(chan sharedTypes.PathName, parallelUploads)
		uploadEg, uploadCtx := errgroup.WithContext(pCtx)
		for i := 0; i < parallelUploads; i++ {
			uploadEg.Go(func() error {
				for path := range uploadQueue {
					f := openReader[path]
					delete(openReader, path)
					parent, _ := t.CreateParents(path.Dir())
					fileRef := parent.GetFile(path.Filename())
					err := m.fm.SendStreamForProjectFile(
						uploadCtx,
						p.Id,
						fileRef.Id,
						f,
						objectStorage.SendOptions{
							ContentSize: fileRef.Size,
						},
					)
					errClose := f.Close()
					if err != nil {
						return errors.Tag(err, "cannot upload file")
					}
					if errClose != nil {
						return errors.Tag(errClose, "cannot close file")
					}
				}
				return nil
			})
		}
		eg.Go(func() error {
			<-uploadCtx.Done()
			// Purge queue as soon as any consumer fails/ctx gets cancelled.
			for range uploadQueue {
			}
			return nil
		})
		eg.Go(func() error {
			err := uploadEg.Wait()
			if err != nil {
				return errors.Merge(err, m.purgeFilestoreData(p.Id))
			}
			return err
		})
		eg.Go(func() error {
			for path := range openReader {
				uploadQueue <- path
			}
			close(uploadQueue)
			return nil
		})
		eg.Go(func() error {
			if err := getProjectNames.Wait(pCtx); err != nil {
				return errors.Tag(err, "cannot get project names")
			}
			p.Name = existingProjectNames.MakeUnique(p.Name)
			if err := m.pm.FinalizeProjectCreation(pCtx, p); err != nil {
				return errors.Tag(err, "cannot finalize project")
			}
			return nil
		})
		if err := eg.Wait(); err != nil {
			return err
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
