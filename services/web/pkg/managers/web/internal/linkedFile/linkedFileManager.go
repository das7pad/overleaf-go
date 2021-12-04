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

package linkedFile

import (
	"context"
	"strings"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	"github.com/das7pad/overleaf-go/services/filestore/pkg/managers/filestore"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/compile"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/fileTree"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/linkedURLProxy"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	CreateLinkedFile(ctx context.Context, request *types.CreateLinkedFileRequest) error
	RefreshLinkedFile(ctx context.Context, request *types.RefreshLinkedFileRequest) error
}

func New(options *types.Options, pm project.Manager, dum documentUpdater.Manager, fm filestore.Manager, cm compile.Manager, ftm fileTree.Manager, proxy linkedURLProxy.Manager) (Manager, error) {
	base, err := sharedTypes.ParseAndValidateURL(
		string(options.PDFDownloadDomain),
	)
	if err != nil {
		return nil, err
	}
	return &manager{
		cm:              cm,
		dum:             dum,
		fm:              fm,
		ftm:             ftm,
		pdfDownloadBase: *base,
		pm:              pm,
		proxy:           proxy,
	}, nil
}

type manager struct {
	cm              compile.Manager
	dum             documentUpdater.Manager
	fm              filestore.Manager
	ftm             fileTree.Manager
	pdfDownloadBase sharedTypes.URL
	pm              project.Manager
	proxy           linkedURLProxy.Manager
}

func (m *manager) CreateLinkedFile(ctx context.Context, request *types.CreateLinkedFileRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}

	switch request.Provider {
	case project.LinkedFileProviderURL:
		return m.fromURL(ctx, request)
	case project.LinkedFileProviderProjectFile:
		return m.fromProjectFile(ctx, request)
	case project.LinkedFileProviderProjectOutputFile:
		return m.fromProjectOutputFile(ctx, request)
	default:
		return &errors.ValidationError{Msg: "unknown provider"}
	}
}

func (m *manager) RefreshLinkedFile(ctx context.Context, request *types.RefreshLinkedFileRequest) error {
	t, _, err := m.pm.GetProjectRootFolder(ctx, request.ProjectId)
	if err != nil {
		return err
	}

	var parentFolder *project.Folder
	var fileRef *project.FileRef
	err = t.WalkFilesMongo(func(folder *project.Folder, e project.TreeElement, path sharedTypes.PathName, mongoPath project.MongoPath) error {
		if e.GetId() == request.FileId {
			parentFolder = folder
			fileRef = e.(*project.FileRef)
			return project.AbortWalk
		}
		return nil
	})
	if err != nil {
		return err
	}
	if fileRef == nil {
		return &errors.NotFoundError{}
	}

	if fileRef.LinkedFileData == nil {
		return &errors.UnprocessableEntityError{Msg: "file is not linked"}
	}

	// The NodeJS implementation stored these as absolute paths.
	if fileRef.LinkedFileData.SourceEntityPath != "" {
		fileRef.LinkedFileData.SourceEntityPath = strings.TrimPrefix(
			fileRef.LinkedFileData.SourceEntityPath, "/",
		)
	}
	if fileRef.LinkedFileData.SourceOutputFilePath != "" {
		fileRef.LinkedFileData.SourceOutputFilePath = strings.TrimPrefix(
			fileRef.LinkedFileData.SourceOutputFilePath, "/",
		)
	}

	request.File = fileRef
	request.ParentFolderId = parentFolder.Id

	switch fileRef.LinkedFileData.Provider {
	case project.LinkedFileProviderURL:
		return m.refreshURL(ctx, request)
	case project.LinkedFileProviderProjectFile:
		return m.refreshProjectFile(ctx, request)
	case project.LinkedFileProviderProjectOutputFile:
		return m.refreshProjectOutputFile(ctx, request)
	default:
		return &errors.ValidationError{Msg: "unknown provider"}
	}
}
