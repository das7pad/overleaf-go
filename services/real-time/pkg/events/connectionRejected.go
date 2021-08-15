// Golang port of the Overleaf real-time service
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

package events

import (
	"github.com/das7pad/overleaf-go/services/real-time/pkg/errors"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

var ConnectionRejectedBadWsBootstrapResponse = &types.RPCResponse{
	Name: "connectionRejected",
	Error: &errors.JavaScriptError{
		Message: "bad wsBootstrap blob",
		Code:    "BadWsBootstrapBlob",
	},
	FatalError: true,
}
var ConnectionRejectedBadWsBootstrapPrepared = prepareBulkMessage(
	ConnectionRejectedBadWsBootstrapResponse,
)

var ConnectionRejectedInternalErrorResponse = &types.RPCResponse{
	Name: "connectionRejected",
	Error: &errors.JavaScriptError{
		Message: "internal error",
	},
	FatalError: true,
}
var ConnectionRejectedInternalErrorPrepared = prepareBulkMessage(
	ConnectionRejectedInternalErrorResponse,
)
