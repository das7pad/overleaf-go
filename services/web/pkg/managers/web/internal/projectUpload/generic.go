// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/constants"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/pendingOperation"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/fileTree"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

const (
	parallelUploads = 5
	retryUploads    = 3
)

type uploadQueueEntry struct {
	file         types.CreateProjectFile
	f            io.ReadCloser
	sourceFileId sharedTypes.UUID
	fileRef      *project.FileRef
}

func tryReuseReader(file types.CreateProjectFile, f io.ReadCloser) (io.ReadCloser, error) {
	if f != nil {
		if seeker, ok := f.(io.Seeker); ok {
			if _, err := seeker.Seek(0, io.SeekStart); err == nil {
				return f, nil
			}
			// Fall back to close and re-open.
		}
		if err := f.Close(); err != nil {
			return nil, errors.Tag(err, "close file")
		}
	}
	newF, _, err := file.Open()
	if err != nil {
		return nil, errors.Tag(err, "re-open file")
	}
	return newF, nil
}

func (m *manager) CreateProject(ctx context.Context, request *types.CreateProjectRequest, response *types.CreateProjectResponse) error {
	if err := request.Validate(); err != nil {
		return err
	}
	p := project.NewProject()
	if err := p.Id.Populate(); err != nil {
		return err
	}

	// Give the project upload 1h until it gets cleaned up by the cron.
	ctx, done := context.WithTimeout(ctx, time.Hour)
	defer done()
	p.DeletedAt = time.Now().
		Add(-constants.ExpireProjectsAfter).
		Add(time.Hour)

	p.CreatedAt = time.Now().Truncate(time.Microsecond)
	if request.Compiler != "" {
		p.Compiler = request.Compiler
	}
	p.ImageName = m.defaultImage
	if request.ImageName != "" {
		p.ImageName = request.ImageName
	}
	p.Name = request.Name
	p.OwnerId = request.UserId
	p.SpellCheckLanguage = request.SpellCheckLanguage

	var existingProjectNames project.Names
	getProjectNames := pendingOperation.TrackOperationWithCancel(
		ctx,
		func(ctx context.Context) error {
			names, err := m.pm.GetProjectNames(ctx, request.UserId)
			if err != nil {
				return errors.Tag(err, "get project names")
			}
			existingProjectNames = names
			return nil
		},
	)
	defer getProjectNames.Cancel()

	fileUploads := make([]uploadQueueEntry, 0, 10)
	defer func() {
		for _, e := range fileUploads {
			if e.f != nil {
				_ = e.f.Close()
			}
		}
	}()
	rootDocPath := request.RootDocPath
	for _, folder := range request.ExtraFolders {
		if _, err := p.RootFolder.CreateParents(folder.Dir()); err != nil {
			return err
		}
	}
	{
		// Prepare tree
		for _, file := range request.Files {
			path := file.Path()
			name := path.Filename()
			parent, errConflict := p.RootFolder.CreateParents(path.Dir())
			if errConflict != nil {
				return errConflict
			}
			if e := file.SourceElement(); e != nil {
				switch el := e.(type) {
				case project.Doc:
					parent.Docs = append(parent.Docs, el)
				case project.FileRef:
					parent.FileRefs = append(parent.FileRefs, el)
					fileUploads = append(fileUploads, uploadQueueEntry{
						sourceFileId: el.Id,
						fileRef:      &parent.FileRefs[len(parent.FileRefs)-1],
					})
				}
				continue
			}
			size := file.Size()
			f, backedByOwnInode, err := file.Open()
			if err != nil {
				return errors.Tag(err, "open file")
			}
			s, isDoc, consumedFile, err := fileTree.IsTextFile(name, size, f)
			if err != nil {
				_ = f.Close()
				return err
			}
			if isDoc {
				if err = f.Close(); err != nil {
					return errors.Tag(err, "close file")
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
				parent.Docs = append(parent.Docs, d)
			} else {
				hash := file.PreComputedHash()
				if hash == "" {
					if consumedFile {
						if f, err = tryReuseReader(file, f); err != nil {
							return err
						}
					}
					if hash, err = fileTree.HashFile(f, size); err != nil {
						_ = f.Close()
						return err
					}
				}
				if backedByOwnInode {
					if err = f.Close(); err != nil {
						return errors.Tag(err, "close file")
					}
					f = nil
				}
				fileRef := project.NewFileRef(name, hash, size)
				fileRef.CreatedAt = time.Now().Truncate(time.Microsecond)
				parent.FileRefs = append(parent.FileRefs, fileRef)
				fileUploads = append(fileUploads, uploadQueueEntry{
					file:    file,
					f:       f,
					fileRef: &parent.FileRefs[len(parent.FileRefs)-1],
				})
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	{
		// Populate ids
		b, err := sharedTypes.GenerateUUIDBulk(p.RootFolder.CountNodes())
		if err != nil {
			return err
		}
		p.RootFolder.PopulateIds(b)

		if rootDocPath != "" {
			parent, _ := p.RootFolder.CreateParents(rootDocPath.Dir())
			p.RootDoc.Doc.Id = parent.GetDoc(rootDocPath.Filename()).Id
		}
	}

	if err := m.pm.PrepareProjectCreation(ctx, &p); err != nil {
		return err
	}

	eg, pCtx := errgroup.WithContext(ctx)
	uploadQueue := make(chan int, parallelUploads)
	uploadEg, uploadCtx := errgroup.WithContext(pCtx)
	for i := 0; i < parallelUploads; i++ {
		uploadEg.Go(func() error {
			for idx := range uploadQueue {
				e := fileUploads[idx]
				mErr := errors.MergedError{}
				for j := 0; j < retryUploads; j++ {
					if err := uploadCtx.Err(); err != nil {
						mErr.Add(err)
						break
					}
					if !e.sourceFileId.IsZero() {
						err := m.fm.CopyProjectFile(
							uploadCtx,
							p.Id, e.fileRef.Id,
							request.SourceProjectId, e.sourceFileId,
						)
						if err != nil {
							mErr.Add(errors.Tag(err, "copy file"))
							continue
						}
						mErr.Clear()
						break
					}
					{
						var err error
						if e.f, err = tryReuseReader(e.file, e.f); err != nil {
							mErr.Add(err)
							break
						}
					}
					err := m.fm.SendStreamForProjectFile(
						uploadCtx, p.Id, e.fileRef.Id, e.f, e.fileRef.Size,
					)
					if err != nil {
						mErr.Add(errors.Tag(err, "upload file"))
						continue
					}
					mErr.Clear()
					break
				}
				if e.f != nil {
					if err := e.f.Close(); err != nil {
						mErr.Add(errors.Tag(err, "close file"))
					}
					e.f = nil // Mark as cleaned up.
				}
				if err := mErr.Finalize(); err != nil {
					return err
				}
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
		for idx := range fileUploads {
			uploadQueue <- idx
		}
		close(uploadQueue)
		return nil
	})
	eg.Go(func() error {
		if err := getProjectNames.Wait(pCtx); err != nil {
			return errors.Tag(err, "get project names")
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
	if err := m.pm.FinalizeProjectCreation(ctx, &p); err != nil {
		return cleanupBestEffort(errors.Tag(err, "finalize project"))
	}

	response.Success = true
	response.ProjectId = &p.Id
	response.Name = p.Name
	return nil
}
