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

package types

import (
	"os"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type CommandLine []string

type Environment []string

type CommandOptions struct {
	CommandLine
	Environment
	sharedTypes.ComputeTimeout
	sharedTypes.ImageName
	sharedTypes.CompileGroup
	CommandOutputFiles
	SetupTime  *sharedTypes.Timed
	SystemTime *sharedTypes.Timed
	UserTime   *sharedTypes.Timed
	WallTime   *sharedTypes.Timed
}

type CommandOutputFiles struct {
	StdErr sharedTypes.PathName `json:"stdErr"`
	StdOut sharedTypes.PathName `json:"stdOut"`
}

func (f *CommandOutputFiles) Cleanup(dir CompileDir) {
	if f.StdErr != "" {
		_ = os.Remove(dir.Join(f.StdErr))
	}
	if f.StdOut != "" {
		_ = os.Remove(dir.Join(f.StdOut))
	}
}

type ExitCode int64
