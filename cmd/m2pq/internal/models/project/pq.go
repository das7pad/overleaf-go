// Golang port of Overleaf
// Copyright (C) 2022 Jakob Ackermann <das7pad@outlook.com>
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

package project

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/lib/pq"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/cmd/m2pq/internal/models/doc"
	"github.com/das7pad/overleaf-go/cmd/m2pq/internal/models/user"
	"github.com/das7pad/overleaf-go/cmd/m2pq/internal/status"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/m2pq"
	"github.com/das7pad/overleaf-go/pkg/objectStorage"
	"github.com/das7pad/overleaf-go/pkg/options/utils"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type DeletedFileDeletedAtField struct {
	DeletedAt time.Time `json:"deletedAt" bson:"deletedAt"`
}

type DeletedFileProjectIdField struct {
	ProjectId primitive.ObjectID `json:"projectId" bson:"projectId"`
}

type DeletedFile struct {
	FileRef                   `bson:"inline"`
	DeletedFileDeletedAtField `bson:"inline"`
	DeletedFileProjectIdField `bson:"inline"`
}

type ForPQ struct {
	ActiveField                      `bson:"inline"`
	ArchivedByField                  `bson:"inline"`
	AuditLogField                    `bson:"inline"`
	CollaboratorRefsField            `bson:"inline"`
	CompilerField                    `bson:"inline"`
	EpochField                       `bson:"inline"`
	IdField                          `bson:"inline"`
	ImageNameField                   `bson:"inline"`
	LastOpenedField                  `bson:"inline"`
	LastUpdatedAtField               `bson:"inline"`
	LastUpdatedByField               `bson:"inline"`
	NameField                        `bson:"inline"`
	OwnerRefField                    `bson:"inline"`
	PublicAccessLevelField           `bson:"inline"`
	ReadOnlyRefsField                `bson:"inline"`
	RootDocIdField                   `bson:"inline"`
	SpellCheckLanguageField          `bson:"inline"`
	TokenAccessReadAndWriteRefsField `bson:"inline"`
	TokenAccessReadOnlyRefsField     `bson:"inline"`
	TokensField                      `bson:"inline"`
	TrashedByField                   `bson:"inline"`
	TreeField                        `bson:"inline"`
	VersionField                     `bson:"inline"`
}

func deserializeDocArchive(r io.ReadCloser) (sharedTypes.Lines, error) {
	blob, err2 := io.ReadAll(r)
	if err2 != nil {
		return nil, errors.Tag(err2, "consume doc archive")
	}
	_ = r.Close()
	{
		var archiveV1 struct {
			Lines sharedTypes.Lines `json:"lines"`
		}
		if err := json.Unmarshal(blob, &archiveV1); err == nil {
			return archiveV1.Lines, nil
		}
	}
	{
		var archiveV0 sharedTypes.Lines
		if err := json.Unmarshal(blob, &archiveV0); err == nil {
			return archiveV0, nil
		}
	}
	return nil, &errors.InvalidStateError{Msg: "unknown archive schema"}
}

type projectFile struct {
	ProjectId sharedTypes.UUID
	FileId    sharedTypes.UUID
}

func Import(ctx context.Context, db *mongo.Database, rTx, tx *sql.Tx, limit int) error {
	var fo objectStorage.Backend
	fBucket := utils.MustGetStringFromEnv("FILESTORE_BUCKET")
	{
		o := objectStorage.Options{}
		utils.ParseJSONFromEnv("FILESTORE_OPTIONS", &o)
		m, err := objectStorage.FromOptions(o)
		if err != nil {
			panic(errors.Tag(err, "create filestore backend"))
		}
		fo = m
	}
	var do objectStorage.Backend
	dBucket := utils.MustGetStringFromEnv("DOCSTORE_BUCKET")
	{
		o := objectStorage.Options{}
		utils.ParseJSONFromEnv("DOCSTORE_OPTIONS", &o)
		m, err := objectStorage.FromOptions(o)
		if err != nil {
			panic(errors.Tag(err, "create docstore backend"))
		}
		do = m
	}

	eg := &errgroup.Group{}
	defer func() {
		// Ensure no clobbering of concurrent retries.
		_ = eg.Wait()
	}()
	copyQueueClosed := false
	copyQueue := make(chan projectFile, 50)

	eg = &errgroup.Group{}
	for j := 0; j < 10; j++ {
		eg.Go(func() error {
			var lastErr error
			for e := range copyQueue {
				dst := e.ProjectId.String() + "/" + e.FileId.String()
				{
					_, err := fo.GetObjectSize(ctx, fBucket, dst)
					if err == nil {
						// already copied in full
						continue
					}
				}
				pId, _ := m2pq.UUID2ObjectID(e.ProjectId)
				fId, _ := m2pq.UUID2ObjectID(e.FileId)
				src :=
					primitive.ObjectID(pId).Hex() +
						"/" +
						primitive.ObjectID(fId).Hex()
				if err := fo.CopyObject(ctx, fBucket, src, dst); err != nil {
					err = errors.Tag(
						err,
						fmt.Sprintf("copy %s -> %s", src, dst),
					)
					log.Println(err.Error())
					lastErr = err
				}
			}
			return lastErr
		})
	}
	defer func() {
		if !copyQueueClosed {
			close(copyQueue)
		}
		for range copyQueue {
		}
	}()

	pQuery := bson.M{}
	dQuery := bson.M{}
	dfQuery := bson.M{}
	{
		var o sharedTypes.UUID
		err := tx.QueryRowContext(ctx, `
SELECT id
FROM projects
ORDER BY id
LIMIT 1
`).Scan(&o)
		if err != nil && err != sql.ErrNoRows {
			return errors.Tag(err, "get last inserted user")
		}
		if err != sql.ErrNoRows {
			oldest, err2 := m2pq.UUID2ObjectID(o)
			if err2 != nil {
				return errors.Tag(err2, "decode last insert id")
			}
			pQuery["_id"] = bson.M{
				"$lt": primitive.ObjectID(oldest),
			}
			dQuery["project_id"] = bson.M{
				"$lt": primitive.ObjectID(oldest),
			}
			dfQuery["projectId"] = bson.M{
				"$lt": primitive.ObjectID(oldest),
			}
		}
	}
	pC, err := db.
		Collection("projects").
		Find(
			ctx,
			pQuery,
			options.Find().
				SetSort(bson.M{"_id": -1}).
				SetBatchSize(100),
		)
	if err != nil {
		return errors.Tag(err, "get project cursor")
	}
	defer func() {
		_ = pC.Close(ctx)
	}()

	dC, err := db.
		Collection("docs").
		Find(
			ctx,
			dQuery,
			options.Find().
				SetSort(bson.M{"project_id": -1}).
				SetBatchSize(100),
		)
	if err != nil {
		return errors.Tag(err, "get docs cursor")
	}
	defer func() {
		_ = dC.Close(ctx)
	}()

	dfC, err := db.
		Collection("deletedFiles").
		Find(
			ctx,
			dQuery,
			options.Find().
				SetSort(bson.M{"projectId": -1}).
				SetBatchSize(100),
		)
	if err != nil {
		return errors.Tag(err, "get deleted files cursor")
	}
	defer func() {
		_ = dfC.Close(ctx)
	}()

	var q *sql.Stmt
	defer func() {
		if q != nil {
			_ = q.Close()
		}
	}()

	auditLogs := make(map[sharedTypes.UUID][]AuditLogEntry)
	maxId, _ := primitive.ObjectIDFromHex("ffffffffffffffffffffffff")
	lastDoc := doc.ForPQ{}
	lastDoc.ProjectId = maxId
	lastDeletedFile := DeletedFile{}
	lastDeletedFile.ProjectId = maxId

	i := 0
	for i = 0; pC.Next(ctx) && i < limit; i++ {
		p := ForPQ{}
		if err = pC.Decode(&p); err != nil {
			return errors.Tag(err, "decode contact")
		}
		pId := m2pq.ObjectID2UUID(p.Id)
		idS := p.Id.Hex()
		log.Printf("projects[%d/%d]: %s", i, limit, idS)

		auditLogs[pId] = p.AuditLog

		for idS < lastDoc.ProjectId.Hex() && dC.Next(ctx) {
			lastDoc = doc.ForPQ{}
			if err = dC.Decode(&lastDoc); err != nil {
				return errors.Tag(err, "decode doc")
			}
		}
		for idS < lastDeletedFile.ProjectId.Hex() && dfC.Next(ctx) {
			lastDeletedFile = DeletedFile{}
			if err = dfC.Decode(&lastDeletedFile); err != nil {
				return errors.Tag(err, "decode deleted file")
			}
		}

		var t *Folder
		if t, err = p.GetRootFolder(); err != nil {
			return errors.Tag(err, "get tree")
		}
		tId := m2pq.ObjectID2UUID(t.Id)

		err = t.WalkFiles(func(e TreeElement, _ sharedTypes.PathName) error {
			f := e.(*FileRef)
			if f.Size != nil {
				return nil
			}
			key := idS + "/" + f.Id.Hex()
			s, err2 := fo.GetObjectSize(ctx, fBucket, key)
			if err2 != nil {
				return errors.Tag(err2, key)
			}
			f.Size = &s
			return nil
		})
		if err != nil {
			return errors.Tag(err, "back fill file size")
		}

		docs := make([]doc.ForPQ, 0)
		for idS == lastDoc.ProjectId.Hex() {
			if lastDoc.InS3 {
				key := idS + "/" + lastDoc.Id.Hex()
				_, r, err2 := do.GetReadStream(
					ctx, dBucket, key, objectStorage.GetOptions{},
				)
				if err2 != nil {
					return errors.Tag(err2, "get doc archive: "+key)
				}
				lastDoc.Lines, err = deserializeDocArchive(r)
				if err != nil {
					return errors.Tag(err, key)
				}
			}
			docs = append(docs, lastDoc)

			if !dC.Next(ctx) {
				break
			}
			lastDoc = doc.ForPQ{}
			if err = dC.Decode(&lastDoc); err != nil {
				return errors.Tag(err, "decode next doc")
			}
		}

		deletedFiles := make([]DeletedFile, 0)
		for idS == lastDeletedFile.ProjectId.Hex() {
			if lastDeletedFile.Size == nil {
				key := idS + "/" + lastDeletedFile.Id.Hex()
				s, err2 := fo.GetObjectSize(ctx, fBucket, key)
				if err2 != nil {
					return errors.Tag(err2, "get deleted file size: "+key)
				}
				lastDeletedFile.Size = &s
			}
			deletedFiles = append(deletedFiles, lastDeletedFile)

			if !dfC.Next(ctx) {
				break
			}
			lastDeletedFile = DeletedFile{}
			if err = dfC.Decode(&lastDeletedFile); err != nil {
				return errors.Tag(err, "decode next file")
			}
		}

		_, err = tx.ExecContext(ctx, `
INSERT INTO projects
(compiler, deleted_at, epoch, id, image_name, last_opened_at, last_updated_at,
 last_updated_by, name, owner_id, public_access_level, spell_check_language,
 token_ro, token_rw, token_rw_prefix, tree_version, root_folder_id,
 root_doc_id)
SELECT $1,
       NULL,
       $2,
       $3,
       $4,
       $5,
       $6,
       (SELECT id from users WHERE id = $7),
       $8,
       $9,
       $10,
       $11,
       nullif($12, ''),
       nullif($13, ''),
       nullif($14, ''),
       $15,
       NULL,
       NULL
`,
			p.Compiler, p.Epoch, pId, p.ImageName, p.LastOpened,
			p.LastUpdatedAt, m2pq.ObjectID2UUID(p.LastUpdatedBy), p.Name,
			m2pq.ObjectID2UUID(p.OwnerRef), p.PublicAccessLevel,
			p.SpellCheckLanguage, p.Tokens.ReadOnly, p.Tokens.ReadAndWrite,
			p.Tokens.ReadAndWritePrefix, p.Version)
		if err != nil {
			return errors.Tag(err, "insert project")
		}

		q, err = tx.PrepareContext(
			ctx,
			pq.CopyIn(
				"tree_nodes",
				"deleted_at", "id", "kind", "parent_id", "path",
				"project_id",
			),
		)
		if err != nil {
			return errors.Tag(err, "prepare tree")
		}

		deletedAt := "1970-01-01"
		_, err = q.ExecContext(
			ctx, deletedAt, tId, "folder", nil, "", pId,
		)
		if err != nil {
			return errors.Tag(err, "queue root folder")
		}

		err = t.WalkFolders(func(f *Folder, path sharedTypes.DirName) error {
			fId := m2pq.ObjectID2UUID(f.Id)
			for _, d := range f.Docs {
				_, err = q.ExecContext(
					ctx,
					deletedAt, m2pq.ObjectID2UUID(d.Id), "doc", fId,
					path.Join(d.Name), pId,
				)
				if err != nil {
					return err
				}
			}
			for _, r := range f.FileRefs {
				fileId := m2pq.ObjectID2UUID(r.Id)
				_, err = q.ExecContext(
					ctx,
					deletedAt, fileId, "file", fId, path.Join(r.Name), pId,
				)
				if err != nil {
					return err
				}
				copyQueue <- projectFile{ProjectId: pId, FileId: fileId}
			}
			for _, ff := range f.Folders {
				_, err = q.ExecContext(
					ctx,
					deletedAt, m2pq.ObjectID2UUID(ff.Id), "folder", fId,
					path.Join(ff.Name)+"/", pId,
				)
				if err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return errors.Tag(err, "queue tree")
		}

		for _, f := range deletedFiles {
			fileId := m2pq.ObjectID2UUID(f.Id)
			_, err = q.ExecContext(
				ctx,
				f.DeletedAt, fileId, "file", tId, f.Name, pId,
			)
			if err != nil {
				return errors.Tag(err, "queue deleted file tree node")
			}
			copyQueue <- projectFile{ProjectId: pId, FileId: fileId}
		}

		for _, d := range docs {
			if !d.Deleted {
				continue
			}
			_, err = q.ExecContext(
				ctx,
				d.DeletedAt, m2pq.ObjectID2UUID(d.Id), "doc", tId, d.Name, pId,
			)
			if err != nil {
				return errors.Tag(err, "queue deleted doc tree node")
			}
		}
		if _, err = q.ExecContext(ctx); err != nil {
			return errors.Tag(err, "flush tree")
		}
		if err = q.Close(); err != nil {
			return errors.Tag(err, "close tree")
		}

		q, err = tx.PrepareContext(
			ctx,
			pq.CopyIn("docs", "id", "snapshot", "version"),
		)
		if err != nil {
			return errors.Tag(err, "prepare docs")
		}
		var rootDocId interface{} = nil
		for _, d := range docs {
			if d.Id == p.RootDocId {
				rootDocId = m2pq.ObjectID2UUID(d.Id)
			}
			_, err = q.ExecContext(
				ctx,
				m2pq.ObjectID2UUID(d.Id), strings.Join(d.Lines, "\n"),
				d.Version,
			)
			if err != nil {
				return errors.Tag(err, "queue doc")
			}
		}
		if _, err = q.ExecContext(ctx); err != nil {
			return errors.Tag(err, "flush docs")
		}
		if err = q.Close(); err != nil {
			return err
		}

		q, err = tx.PrepareContext(
			ctx,
			pq.CopyIn(
				"files",
				"id", "created_at", "hash", "linked_file_data", "size", "pending",
			),
		)
		if err != nil {
			return errors.Tag(err, "prepare files")
		}
		err = t.WalkFiles(func(e TreeElement, _ sharedTypes.PathName) error {
			d := e.(*FileRef)
			_, err = q.ExecContext(
				ctx,
				m2pq.ObjectID2UUID(d.Id), d.Created, d.Hash, d.LinkedFileData,
				*d.Size,
				false,
			)
			return err
		})
		if err != nil {
			return errors.Tag(err, "queue files")
		}
		for _, f := range deletedFiles {
			_, err = q.ExecContext(
				ctx,
				m2pq.ObjectID2UUID(f.Id), f.Created, f.Hash, f.LinkedFileData,
				*f.Size,
				false,
			)
			if err != nil {
				return errors.Tag(err, "queue deleted file")
			}
		}
		if _, err = q.ExecContext(ctx); err != nil {
			return errors.Tag(err, "flush files")
		}
		if err = q.Close(); err != nil {
			return errors.Tag(err, "close files")
		}

		_, err = tx.ExecContext(ctx, `
UPDATE projects
SET deleted_at     = NULL,
    root_doc_id    = $2,
    root_folder_id = $3
WHERE id = $1
`, pId, rootDocId, tId)
		if err != nil {
			return errors.Tag(err, "finalize project")
		}

		q, err = tx.PrepareContext(
			ctx,
			pq.CopyIn(
				"project_members",
				"project_id", "user_id", "access_source", "privilege_level", "archived", "trashed",
			),
		)
		if err != nil {
			return errors.Tag(err, "prepare collaborators")
		}

		access := []struct {
			AccessSource
			sharedTypes.PrivilegeLevel
			Refs
		}{
			{
				AccessSource:   AccessSourceOwner,
				PrivilegeLevel: sharedTypes.PrivilegeLevelOwner,
				Refs:           Refs{p.OwnerRef},
			},
			{
				AccessSource:   AccessSourceInvite,
				PrivilegeLevel: sharedTypes.PrivilegeLevelReadAndWrite,
				Refs:           p.CollaboratorRefs,
			},
			{
				AccessSource:   AccessSourceInvite,
				PrivilegeLevel: sharedTypes.PrivilegeLevelReadOnly,
				Refs:           p.ReadOnlyRefs,
			},
			{
				AccessSource:   AccessSourceToken,
				PrivilegeLevel: sharedTypes.PrivilegeLevelReadAndWrite,
				Refs:           p.TokenAccessReadAndWriteRefs,
			},
			{
				AccessSource:   AccessSourceToken,
				PrivilegeLevel: sharedTypes.PrivilegeLevelReadOnly,
				Refs:           p.TokenAccessReadOnlyRefs,
			},
		}
		seen := make(map[primitive.ObjectID]bool, 0)

		for _, a := range access {
			for _, userId := range a.Refs {
				if seen[userId] {
					continue
				}
				_, err = q.ExecContext(
					ctx,
					pId, m2pq.ObjectID2UUID(userId), a.AccessSource,
					a.PrivilegeLevel, p.ArchivedBy.Contains(userId),
					p.TrashedBy.Contains(userId),
				)
				if err != nil {
					return errors.Tag(err, "queue collaborator")
				}
				seen[userId] = true
			}
		}

		if _, err = q.ExecContext(ctx); err != nil {
			return errors.Tag(err, "flush collaborator queue")
		}
		if err = q.Close(); err != nil {
			return errors.Tag(err, "close collaborator queue")
		}
	}

	nAuditLogs := 0
	initiatorMongoIds := make(map[primitive.ObjectID]bool)
	for _, entries := range auditLogs {
		nAuditLogs += len(entries)
		for _, entry := range entries {
			initiatorMongoIds[entry.InitiatorId] = true
		}
	}
	initiatorIds, err := user.ResolveUsers(ctx, rTx, initiatorMongoIds)
	if err != nil {
		return errors.Tag(err, "resolve audit log users")
	}

	q, err = tx.PrepareContext(
		ctx,
		pq.CopyIn(
			"project_audit_log",
			"id", "info", "initiator_id", "operation", "project_id", "timestamp",
		),
	)
	if err != nil {
		return errors.Tag(err, "prepare audit log")
	}

	ids, err := sharedTypes.GenerateUUIDBulk(nAuditLogs)
	if err != nil {
		return errors.Tag(err, "audit log ids")
	}
	for projectId, entries := range auditLogs {
		for _, entry := range entries {
			var infoBlob []byte
			infoBlob, err = json.Marshal(entry.Info)
			if err != nil {
				return errors.Tag(err, "serialize audit log")
			}
			_, err = q.ExecContext(
				ctx,
				ids.Next(),                      // id
				string(infoBlob),                // info
				initiatorIds[entry.InitiatorId], // initiator_id
				entry.Operation,                 // operation
				projectId,                       // project_id
				entry.Timestamp,                 // timestamp
			)
			if err != nil {
				return errors.Tag(err, "queue audit log")
			}
		}
	}
	if _, err = q.ExecContext(ctx); err != nil {
		return errors.Tag(err, "flush audit log queue")
	}
	if err = q.Close(); err != nil {
		return errors.Tag(err, "close audit log queue")
	}

	// Upon returning for committing the tx, all copying should have finished.
	copyQueueClosed = true
	close(copyQueue)
	if err = eg.Wait(); err != nil {
		return err
	}

	if i == limit {
		return status.HitLimit
	}
	return nil
}
