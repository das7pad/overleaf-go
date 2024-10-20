// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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

//goland:noinspection SpellCheckingInspection
const (
	AgentErrorName           = ".agent-error"
	AgentSocketName          = ".agent-socket"
	AgentErrorPathContainer  = CompileDirContainer + "/" + AgentErrorName
	AgentSocketPathContainer = CompileDirContainer + "/" + AgentSocketName

	CompileDirContainer = "/compile"
	OutputDirContainer  = "/output"

	CompileDirPlaceHolder = "$COMPILE_DIR"
	OutputDirPlaceHolder  = "$OUTPUT_DIR"

	CompileOutputLabel        = "compile-output"
	ContentLabel              = "content"
	ProjectSyncStateFilename  = ".project-sync-state-v3"
	PDFCachingTrackerFilename = "pdf-caching.json"
	PDFCachingXrefFilename    = "output.pdfxref"

	TimedOut       = "timedout"
	Cancelled      = "cancelled"
	Terminated     = "terminated"
	Success        = "success"
	Failure        = "failure"
	ValidationPass = "validation-pass"
	ValidationFail = "validation-fail"
)
