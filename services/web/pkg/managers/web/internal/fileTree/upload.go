// Golang port of Overleaf
// Copyright (C) 2021-2023 Jakob Ackermann <das7pad@outlook.com>
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

package fileTree

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"io"
	"strconv"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	documentUpdaterTypes "github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) UploadFile(ctx context.Context, request *types.UploadFileRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	folderId := request.ParentFolderId
	projectId := request.ProjectId
	userId := request.UserId
	source := "upload"

	var isDoc bool
	var s sharedTypes.Snapshot
	if request.LinkedFileData != nil {
		isDoc = false
	} else {
		var err error
		var consumedFile bool
		s, isDoc, consumedFile, err = IsTextFile(
			request.FileName, request.Size, request.File,
		)
		if err != nil {
			return err
		}
		if consumedFile && !isDoc {
			if err = request.SeekFileToStart(); err != nil {
				return err
			}
		}
	}
	var hash sharedTypes.Hash
	if !isDoc {
		var err error
		if hash, err = HashFile(request.File, request.Size); err != nil {
			return err
		}
	}

	var err error
	var existingId sharedTypes.UUID
	var existingIsDoc bool
	var uploadedFileRef *project.FileRef
	var uploadedDoc *project.Doc
	var v sharedTypes.Version

	if isDoc {
		doc := project.NewDoc(request.FileName)
		doc.Snapshot = string(s)
		if err = sharedTypes.PopulateUUID(&doc.Id); err != nil {
			return err
		}
		existingId, existingIsDoc, v, err = m.pm.EnsureIsDoc(
			ctx, projectId, userId, folderId, &doc,
		)
		if err != nil {
			return errors.Tag(err, "cannot create populated doc")
		}
		if !existingId.IsZero() && existingIsDoc {
			// This a text file upload on a doc. Just upsert the content.
			err = m.dum.SetDoc(
				ctx, projectId, existingId,
				documentUpdaterTypes.SetDocRequest{
					Snapshot: s,
					Source:   source,
					UserId:   userId,
				},
			)
			if err != nil {
				return errors.Tag(err, "cannot upsert doc")
			}
			_ = m.projectMetadata.BroadcastMetadataForDocFromSnapshot(
				projectId, existingId, doc.Snapshot,
			)
			return nil
		}
		_ = m.projectMetadata.BroadcastMetadataForDocFromSnapshot(
			projectId, doc.Id, doc.Snapshot,
		)
		uploadedDoc = &doc
	} else {
		if err = request.SeekFileToStart(); err != nil {
			return err
		}
		file := project.NewFileRef(request.FileName, hash, request.Size)
		file.CreatedAt = time.Now().Truncate(time.Microsecond)
		file.LinkedFileData = request.LinkedFileData
		if err = sharedTypes.PopulateUUID(&file.Id); err != nil {
			return err
		}
		uploadCtx, done := context.WithTimeout(ctx, fileUploadsStaleAfter)
		defer done()
		err = m.pm.PrepareFileCreation(
			uploadCtx, projectId, userId, folderId, &file,
		)
		if err != nil {
			return errors.Tag(err, "prepare tree entry")
		}
		err = m.fm.SendStreamForProjectFile(
			uploadCtx,
			projectId,
			file.Id,
			request.File,
			request.Size,
		)
		if err != nil {
			return errors.Tag(err, "cannot upload new file")
		}
		existingId, existingIsDoc, v, err = m.pm.FinalizeFileCreation(
			uploadCtx, projectId, userId, &file,
		)
		if err != nil {
			return errors.Tag(err, "finalize file creation")
		}
		uploadedFileRef = &file
	}

	// Any dangling elements have been deleted and new ones created.
	// Failing the request and retrying now would result in duplicates.
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	if !existingId.IsZero() {
		if existingIsDoc {
			m.cleanupDocDeletion(ctx, projectId, existingId)
		}
		m.notifyEditor(
			projectId, "removeEntity",
			existingId, source, v,
		)
	}
	if f := uploadedFileRef; f != nil {
		//goland:noinspection SpellCheckingInspection
		m.notifyEditor(
			projectId, "reciveNewFile",
			folderId, f, source, f.LinkedFileData, userId, v,
		)
	} else {
		uploadedDoc.Snapshot = ""
		//goland:noinspection SpellCheckingInspection
		m.notifyEditor(
			projectId, "reciveNewDoc",
			folderId, uploadedDoc, v,
		)
	}
	return nil
}

func HashFile(reader io.Reader, size int64) (sharedTypes.Hash, error) {
	d := sha1.New()
	d.Write([]byte(
		"blob " + strconv.FormatInt(size, 10) + "\x00",
	))
	if _, err := io.Copy(d, reader); err != nil {
		return "", errors.Tag(err, "cannot compute hash")
	}
	return sharedTypes.Hash(hex.EncodeToString(d.Sum(nil))), nil
}
