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

package openInOverleaf

import (
	"bytes"
	"context"
	"io"

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type projectFile struct {
	s *types.OpenInOverleafSnippet
}

func (f *projectFile) Size() int64 {
	if f.s.File != nil {
		return f.s.File.Size()
	}
	return int64(len(string(f.s.Snapshot)))
}

func (f *projectFile) Path() sharedTypes.PathName {
	if f.s.Path != "" || f.s.File == nil {
		return f.s.Path
	}
	return f.s.File.Path()
}

func (f *projectFile) Open() (io.ReadCloser, error) {
	if f.s.File != nil {
		return f.s.File.Open()
	}
	return io.NopCloser(bytes.NewBuffer([]byte(string(f.s.Snapshot)))), nil
}

func (f *projectFile) PreComputedHash() sharedTypes.Hash {
	return ""
}

const parallelDownloads = 5

func (m *manager) createFromSnippets(ctx context.Context, request *types.OpenInOverleafRequest, response *types.CreateProjectResponse) error {
	defer func() {
		for _, snippet := range request.Snippets {
			if f := snippet.File; f != nil {
				f.Cleanup()
			}
		}
	}()

	downloadQueue := make(chan *types.OpenInOverleafSnippet, parallelDownloads)
	readyQueue := make(chan *types.OpenInOverleafSnippet, parallelDownloads)
	eg, pCtx := errgroup.WithContext(ctx)
	go func() {
		<-pCtx.Done()
		if pCtx.Err() != nil {
			for range downloadQueue {
				// Flush the queue on error.
			}
		}
	}()
	eg.Go(func() error {
		for _, snippet := range request.Snippets {
			if snippet.URL != nil {
				downloadQueue <- snippet
			} else {
				if snippet.Path == "main.tex" {
					snippet.Snapshot = addDocumentClass(
						snippet.Snapshot, request.ProjectName,
					)
				}
				readyQueue <- snippet
			}
		}
		close(downloadQueue)
		return nil
	})
	uploadEg, uploadCtx := errgroup.WithContext(pCtx)
	for i := 0; i < parallelDownloads; i++ {
		uploadEg.Go(func() error {
			for snippet := range downloadQueue {
				f, err := m.proxy.DownloadFile(uploadCtx, snippet.URL)
				if err != nil {
					return err
				}
				snippet.File = f
				readyQueue <- snippet
			}
			return nil
		})
	}
	files := make([]types.CreateProjectFile, 0, len(request.Snippets))
	eg.Go(func() error {
		for snippet := range readyQueue {
			files = append(files, &projectFile{s: snippet})
		}
		return nil
	})
	eg.Go(func() error {
		err := uploadEg.Wait()
		close(readyQueue)
		return err
	})
	if err := eg.Wait(); err != nil {
		return err
	}

	return m.pum.CreateProject(ctx, &types.CreateProjectRequest{
		AddHeader:      m.addHeader,
		Compiler:       request.Compiler,
		Files:          files,
		HasDefaultName: request.HasDefaultName,
		Name:           request.ProjectName,
		UserId:         request.Session.User.Id,
	}, response)
}
