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

import (
	"os"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type CommandLine []string

type Environment []string

type Timeout time.Duration

const MaxTimeout = Timeout(10 * time.Minute)

var maxTimeoutPretty = time.Duration(MaxTimeout).String()

func (t Timeout) Validate() error {
	if t <= 0 {
		return &errors.ValidationError{Msg: "timeout must be greater zero"}
	}
	if t > MaxTimeout {
		return &errors.ValidationError{
			Msg: "timeout must be below " + maxTimeoutPretty,
		}
	}
	return nil
}

type CommandOptions struct {
	CommandLine
	Environment
	Timeout
	ImageName
	CompileGroup
	CommandOutputFiles
	*sharedTypes.Timed
}

type CommandOutputFiles struct {
	StdErr sharedTypes.PathName
	StdOut sharedTypes.PathName
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
