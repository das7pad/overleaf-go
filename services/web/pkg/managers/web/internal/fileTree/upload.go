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
	"github.com/das7pad/overleaf-go/pkg/objectStorage"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	documentUpdaterTypes "github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) UploadFile(ctx context.Context, request *types.UploadFileRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	parentFolderId := request.ParentFolderId
	projectId := request.ProjectId
	userId := request.UserId
	source := "upload"

	s, isDoc, errIsTextFile := isTextFile(request)
	if errIsTextFile != nil {
		return errIsTextFile
	}

	var v sharedTypes.Version
	var hash sharedTypes.Hash
	var upsertDoc *project.Doc
	var fsPath sharedTypes.PathName
	var uploadedFileRef *project.FileRef
	var deletedElement, newElement project.TreeElement

	err := m.txWithRetries(ctx, func(ctx context.Context) error {
		p := &project.WithTreeAndRootDoc{}
		if err := m.pm.GetProject(ctx, projectId, p); err != nil {
			return errors.Tag(err, "cannot get project")
		}
		v = p.Version
		t, err := p.GetRootFolder()
		if err != nil {
			return err
		}
		var parentFolder *project.Folder
		var mongoPath project.MongoPath
		err = t.WalkFoldersMongo(func(_, f *project.Folder, d sharedTypes.DirName, mPath project.MongoPath) error {
			if f.Id == parentFolderId {
				mongoPath = mPath
				fsPath = d.Join(request.FileName)
				parentFolder = f
				return project.AbortWalk
			}
			return nil
		})
		if err != nil {
			return err
		}
		if parentFolder == nil {
			return errors.Tag(&errors.NotFoundError{}, "unknown folder_id")
		}

		// Delete any conflicting entries -- or update in-place and bail-out.
		if entry, mp := parentFolder.GetEntry(request.FileName); entry != nil {
			switch e := entry.(type) {
			case *project.Doc:
				if isDoc {
					upsertDoc = e
					// all done in mongo at this point, bail-out.
					return nil
				} else {
					deletedElement = entry
					err = m.deleteDocFromProject(
						ctx, projectId, v, p.RootDocId, mongoPath+mp, e,
					)
					if err != nil {
						return errors.Tag(
							err, "cannot delete doc for overwriting",
						)
					}
					v++
					// upload as new file
				}
			case *project.FileRef:
				deletedElement = entry
				err = m.deleteFileFromProject(
					ctx, projectId, v, mongoPath+mp, e,
				)
				if err != nil {
					return errors.Tag(
						err, "cannot delete file for overwriting",
					)
				}
				v++
				// upload as new doc/file
			case *project.Folder:
				return &errors.InvalidStateError{
					Msg: "cannot overwrite folder",
				}
			}
		}

		// Create the new element.
		if isDoc {
			doc := project.NewDoc(request.FileName)
			_, _, err = m.dm.UpdateDoc(
				ctx,
				projectId,
				doc.Id,
				s.ToLines(),
				0,
				sharedTypes.Ranges{},
			)
			if err != nil {
				return errors.Tag(err, "cannot create populated doc")
			}
			newElement = doc
		} else if uploadedFileRef == nil {
			// Upload once.
			if hash == "" {
				// Hash once.
				hash, err = hashFile(request)
				if err != nil {
					return errors.Tag(err, "cannot compute hash")
				}
			}
			fileRef := project.NewFileRef(
				request.FileName,
				hash,
				request.Size,
			)
			err = request.SeekFileToStart()
			if err != nil {
				return err
			}
			err = m.fm.SendStreamForProjectFile(
				ctx,
				projectId,
				fileRef.Id,
				request.File,
				objectStorage.SendOptions{
					ContentSize: request.Size,
				},
			)
			if err != nil {
				return errors.Tag(err, "cannot create new file")
			}
			newElement = fileRef
			uploadedFileRef = fileRef
		}
		err = m.pm.AddTreeElement(ctx, projectId, v, mongoPath, newElement)
		if err != nil {
			return errors.Tag(err, "cannot add element into tree")
		}
		v++
		return nil
	})
	if err != nil {
		if uploadedFileRef != nil {
			cCtx, done := context.WithTimeout(
				context.Background(), 10*time.Second,
			)
			defer done()
			_ = m.fm.DeleteProjectFile(cCtx, projectId, uploadedFileRef.Id)
		}
		return err
	}
	if upsertDoc != nil {
		err = m.dum.SetDoc(
			ctx, projectId, upsertDoc.Id, &documentUpdaterTypes.SetDocRequest{
				Snapshot: s,
				Source:   source,
				UserId:   userId,
				Undoing:  false,
			},
		)
		if err != nil {
			return errors.Tag(err, "cannot upsert doc")
		}
	}

	// Any dangling elements have been deleted and new ones created.
	// Failing the request and retrying now would result in duplicates.
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	if deletedElement != nil {
		if doc, ok := deletedElement.(*project.Doc); ok {
			m.cleanupDocDeletion(ctx, projectId, doc.Id)
		}
		go m.notifyEditor(
			projectId, "removeEntity",
			deletedElement.GetId(), source, v,
		)
	}
	if f, ok := newElement.(*project.FileRef); ok {
		//goland:noinspection SpellCheckingInspection
		go m.notifyEditor(
			projectId, "reciveNewFile",
			parentFolderId, newElement, source, f.LinkedFileData, userId, v,
		)
	}
	if _, ok := newElement.(*project.Doc); ok {
		//goland:noinspection SpellCheckingInspection
		go m.notifyEditor(
			projectId, "reciveNewDoc",
			parentFolderId, newElement, v,
		)
	}
	if upsertDoc != nil {
		_ = m.projectMetadata.BroadcastMetadataForDoc(
			ctx, projectId, upsertDoc.Id,
		)
	}
	return nil
}

func hashFile(request *types.UploadFileRequest) (sharedTypes.Hash, error) {
	d := sha1.New()
	d.Write([]byte(
		"blob " + strconv.FormatInt(request.Size, 10) + "\x00",
	))
	if err := request.SeekFileToStart(); err != nil {
		return "", err
	}
	if _, err := io.Copy(d, request.File); err != nil {
		return "", errors.Tag(err, "cannot hash data")
	}
	return sharedTypes.Hash(hex.EncodeToString(d.Sum(nil))), nil
}