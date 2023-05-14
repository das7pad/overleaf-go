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

package events

import (
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

var ConnectionRejectedBadWsBootstrapPrepared = mustPrepareBulkMessageOffline(
	&types.RPCResponse{
		Name:       "connectionRejected",
		FatalError: true,
	},
	errors.JavaScriptError{
		Message: "bad wsBootstrap blob",
		Code:    "BadWsBootstrapBlob",
	},
)

var ConnectionRejectedInternalErrorPrepared = mustPrepareBulkMessageOffline(
	&types.RPCResponse{
		Name:       "connectionRejected",
		FatalError: true,
	},
	errors.JavaScriptError{
		Message: "internal error",
	},
)

var ConnectionRejectedRetryPrepared = mustPrepareBulkMessageOffline(
	&types.RPCResponse{
		Name:       "connectionRejected",
		FatalError: true,
	},
	errors.JavaScriptError{
		Message: "retry",
	},
)

var ReconnectGracefullyPrepared = mustPrepareBulkMessageOffline(
	&types.RPCResponse{
		Name: "reconnectGracefully",
	},
	nil,
)
