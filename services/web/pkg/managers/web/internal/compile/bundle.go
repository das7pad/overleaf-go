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

package compile

import (
	"context"
	"time"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type ClsiManager interface {
	CleanupOldProjects(ctx context.Context, threshold time.Time) error

	ClearCache(projectId sharedTypes.UUID, userId sharedTypes.UUID) error

	Compile(
		ctx context.Context,
		projectId sharedTypes.UUID,
		userId sharedTypes.UUID,
		request *clsiTypes.CompileRequest,
		response *clsiTypes.CompileResponse,
	) error

	GetCapacity() (int64, error)

	HealthCheck(ctx context.Context) error

	PeriodicCleanup(ctx context.Context)

	StartInBackground(ctx context.Context, projectId, userId sharedTypes.UUID, request *clsiTypes.StartInBackgroundRequest) error

	SyncFromCode(
		ctx context.Context,
		projectId sharedTypes.UUID,
		userId sharedTypes.UUID,
		request *clsiTypes.SyncFromCodeRequest,
		positions *clsiTypes.PDFPositions,
	) error

	SyncFromPDF(
		ctx context.Context,
		projectId sharedTypes.UUID,
		userId sharedTypes.UUID,
		request *clsiTypes.SyncFromPDFRequest,
		positions *clsiTypes.CodePositions,
	) error

	WordCount(
		ctx context.Context,
		projectId sharedTypes.UUID,
		userId sharedTypes.UUID,
		request *clsiTypes.WordCountRequest,
		words *clsiTypes.Words,
	) error
}
