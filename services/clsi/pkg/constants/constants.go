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

package constants

const AgentSocketName = ".agent-socket"
const AgentSocketPathContainer = CompileDirContainer + "/" + AgentSocketName

const CompileDirContainer = "/compile"
const OutputDirContainer = "/output"

const CompileDirPlaceHolder = "$COMPILE_DIR"
const OutputDirPlaceHolder = "$OUTPUT_DIR"

const CompileOutputLabel = "compile-output"
const ProjectSyncStateFilename = ".project-sync-state"

const Cancelled = "cancelled"
const TimedOut = "timedout"
const Terminated = "terminated"
const Success = "success"
const Failure = "failure"
const ValidationPass = "validation-pass"
const ValidationFail = "validation-fail"
