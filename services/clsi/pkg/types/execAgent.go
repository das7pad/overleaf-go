// Golang port of the Overleaf clsi service
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

package types

type ExecAgentRequestOptions struct {
	CommandLine        `json:"command_line"`
	Environment        `json:"environment"`
	Timeout            `json:"timeout"`
	CommandOutputFiles `json:"command_output_files"`
}

type ExecAgentResponseBody struct {
	ExitCode     ExitCode `json:"exitCode"`
	ErrorMessage string   `json:"error_message"`
}
