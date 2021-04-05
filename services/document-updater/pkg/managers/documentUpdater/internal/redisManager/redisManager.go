// Golang port of the Overleaf document-updater service
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

package redisManager

import (
	"context"

	"github.com/go-redis/redis/v8"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/document-updater/pkg/types"
)

type Manager interface {
	putDocInMemory(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
		doc types.Doc,
	) error

	removeDocFromMemory(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
	) error

	checkOrSetProjectState(
		ctx context.Context,
		projectId primitive.ObjectID,
		newState string,
	) (bool, error)

	clearProjectState(
		ctx context.Context,
		projectId primitive.ObjectID,
	) error

	getDoc(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
	) (types.Doc, error)

	getDocVersion(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
	) (int64, string, error)

	getPreviousDocOps(
		ctx context.Context,
		docId primitive.ObjectID,
		start int64,
		end int64,
	) ([]types.Op, error)

	getHistoryType(
		ctx context.Context,
		docId primitive.ObjectID,
	) (string, error)

	setHistoryType(
		ctx context.Context,
		docId primitive.ObjectID,
		projectHistoryType string,
	) error

	updateDocument(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
		doc types.Doc,
		appliedOps []types.Op,
		updateMetaData types.DocumentUpdateMeta,
	) error

	renameDoc(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
		userId primitive.ObjectID,
		update types.RenameUpdate,
		projectHistoryId int64,
	) error

	clearUnFlushedTime(
		ctx context.Context,
		docId primitive.ObjectID,
	) error

	getDocIdsInProject(
		ctx context.Context,
		projectId primitive.ObjectID,
	) ([]primitive.ObjectID, error)

	getDocTimestamps(
		ctx context.Context,
		docIds []primitive.ObjectID,
	) ([]int64, error)

	queueFlushAndDeleteProject(
		ctx context.Context,
		projectId primitive.ObjectID,
	) error

	getNextProjectToFlushAndDelete(
		ctx context.Context,
		cutoffTime int64,
	) (string, int64, int64, error)
}

func New(rClient *redis.UniversalClient) Manager {
	return &manager{rClient: rClient}
}

type manager struct {
	rClient *redis.UniversalClient
}

func (m *manager) putDocInMemory(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID, doc types.Doc) error {
	panic("implement me")
}

func (m *manager) removeDocFromMemory(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID) error {
	panic("implement me")
}

func (m *manager) checkOrSetProjectState(ctx context.Context, projectId primitive.ObjectID, newState string) (bool, error) {
	panic("implement me")
}

func (m *manager) clearProjectState(ctx context.Context, projectId primitive.ObjectID) error {
	panic("implement me")
}

func (m *manager) getDoc(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID) (types.Doc, error) {
	panic("implement me")
}

func (m *manager) getDocVersion(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID) (int64, string, error) {
	panic("implement me")
}

func (m *manager) getPreviousDocOps(ctx context.Context, docId primitive.ObjectID, start int64, end int64) ([]types.Op, error) {
	panic("implement me")
}

func (m *manager) getHistoryType(ctx context.Context, docId primitive.ObjectID) (string, error) {
	panic("implement me")
}

func (m *manager) setHistoryType(ctx context.Context, docId primitive.ObjectID, projectHistoryType string) error {
	panic("implement me")
}

func (m *manager) updateDocument(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID, doc types.Doc, appliedOps []types.Op, updateMetaData types.DocumentUpdateMeta) error {
	panic("implement me")
}

func (m *manager) renameDoc(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID, userId primitive.ObjectID, update types.RenameUpdate, projectHistoryId int64) error {
	panic("implement me")
}

func (m *manager) clearUnFlushedTime(ctx context.Context, docId primitive.ObjectID) error {
	panic("implement me")
}

func (m *manager) getDocIdsInProject(ctx context.Context, projectId primitive.ObjectID) ([]primitive.ObjectID, error) {
	panic("implement me")
}

func (m *manager) getDocTimestamps(ctx context.Context, docIds []primitive.ObjectID) ([]int64, error) {
	panic("implement me")
}

func (m *manager) queueFlushAndDeleteProject(ctx context.Context, projectId primitive.ObjectID) error {
	panic("implement me")
}

func (m *manager) getNextProjectToFlushAndDelete(ctx context.Context, cutoffTime int64) (string, int64, int64, error) {
	panic("implement me")
}
