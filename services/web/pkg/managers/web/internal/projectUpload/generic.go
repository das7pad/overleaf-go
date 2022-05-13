// Golang port of Overleaf
// Copyright (C) 2021-2022 Jakob Ackermann <das7pad@outlook.com>
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
	"time"

	"github.com/edgedb/edgedb-go"
	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/objectStorage"
	"github.com/das7pad/overleaf-go/pkg/pendingOperation"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/constants"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/fileTree"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

const (
	parallelUploads = 5
	retryUploads    = 3
)

type uploadQueueEntry struct {
	file         types.CreateProjectFile
	reader       io.ReadCloser
	sourceFileId edgedb.UUID
}

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
	p.DeletedAt = edgedb.NewOptionalDateTime(
		// Give the project upload 1h until it gets cleaned up by the cron.
		time.Now().UTC().Add(-constants.ExpireProjectsAfter).Add(time.Hour),
	)
	if request.Compiler != "" {
		p.Compiler = request.Compiler
	}
	p.ImageName = m.options.DefaultImage
	if request.ImageName != "" {
		p.ImageName = request.ImageName
	}
	p.Name = request.Name
	p.SpellCheckLanguage = request.SpellCheckLanguage

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

	openReader := make(map[sharedTypes.PathName]uploadQueueEntry)
	defer func() {
		for _, e := range openReader {
			if f := e.reader; f != nil {
				_ = f.Close()
			}
		}
	}()
	t := &p.RootFolder
	rootDocPath := request.RootDocPath

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
			if e := file.SourceElement(); e != nil {
				switch el := e.(type) {
				case project.Doc:
					parent.Docs = append(parent.Docs, el)
				case project.FileRef:
					parent.FileRefs = append(parent.FileRefs, el)
					openReader[path] = uploadQueueEntry{
						sourceFileId: el.Id,
					}
				}
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
				openReader[path] = uploadQueueEntry{file: file, reader: f}
			}
		}
		return nil
	})
	if err := eg.Wait(); err != nil {
		if p.Id != (edgedb.UUID{}) {
			return errors.Merge(err, m.pm.HardDelete(ctx, p.Id))
		}
		return err
	}

	if err := m.pm.CreateProjectTree(ctx, p); err != nil {
		return errors.Merge(err, m.pm.HardDelete(ctx, p.Id))
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
				e := openReader[path]
				parent, _ := t.CreateParents(path.Dir())
				fileRef := parent.GetFile(path.Filename())
				mErr := &errors.MergedError{}
				for j := 0; j < retryUploads; j++ {
					if e.reader == nil {
						err := m.fm.CopyProjectFile(
							uploadCtx,
							request.SourceProjectId,
							e.sourceFileId,
							p.Id,
							fileRef.Id,
						)
						if err == nil {
							mErr.Clear()
							break
						}
						mErr.Add(errors.Tag(err, "cannot copy file"))
						continue
					}
					err := m.fm.SendStreamForProjectFile(
						uploadCtx,
						p.Id,
						fileRef.Id,
						e.reader,
						objectStorage.SendOptions{
							ContentSize: fileRef.Size,
						},
					)
					if err == nil {
						mErr.Clear()
						break
					}
					mErr.Add(errors.Tag(err, "cannot upload file"))
					e.reader, err = seekToStart(e.file, e.reader)
					mErr.Add(err)
					continue
				}
				if e.reader != nil {
					if err := e.reader.Close(); err != nil {
						mErr.Add(errors.Tag(err, "cannot close file"))
					}
				}
				if err := mErr.Finalize(); err != nil {
					return err
				}
				delete(openReader, path)
			}
			return nil
		})
	}
	eg.Go(func() error {
		<-uploadCtx.Done()
		// Purge the queue as soon as any consumer fails/ctx gets cancelled.
		for range uploadQueue {
		}
		return nil
	})
	eg.Go(func() error {
		return uploadEg.Wait()
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
		return nil
	})
	cleanupBestEffort := func(err error) error {
		// NOTE: The cron job will clean up behind us if we fail here.
		if errFiles := m.purgeFilestoreData(p.Id); errFiles != nil {
			return err
		}
		return errors.Merge(err, m.pm.HardDelete(ctx, p.Id))
	}

	if err := eg.Wait(); err != nil {
		return cleanupBestEffort(err)
	}
	if err := m.pm.FinalizeProjectCreation(ctx, p); err != nil {
		return cleanupBestEffort(errors.Tag(err, "cannot finalize project"))
	}

	response.Success = true
	response.ProjectId = &p.Id
	return nil
}
