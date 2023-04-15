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

package commandRunner

import (
	"context"
	"os"
	"path"
	"strings"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/constants"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type Runner interface {
	Setup(ctx context.Context, namespace types.Namespace, imageName sharedTypes.ImageName) (*time.Time, error)
	Run(ctx context.Context, namespace types.Namespace, options *types.CommandOptions) (types.ExitCode, error)
	Stop(namespace types.Namespace) error
	Resolve(path string, namespace types.Namespace) (sharedTypes.PathName, error)
}

type NamespacedRun func(ctx context.Context, options *types.CommandOptions) (types.ExitCode, error)

func New(options *types.Options) (Runner, error) {
	switch options.Runner {
	case "agent":
		return newAgentRunner(options)
	case "local":
		return newLocalRunner(options)
	default:
		return nil, errors.New("unknown runner: " + options.Runner)
	}
}

func CreateCommandOutput(dir types.CompileDir) (*types.CommandOutputFiles, error) {
	stdOutHandle, err := os.CreateTemp(string(dir), ".output.*.stdout")
	if err != nil {
		return nil, err
	}
	stdOut := sharedTypes.PathName(path.Base(stdOutHandle.Name()))
	if err = stdOutHandle.Close(); err != nil {
		return nil, err
	}
	stdErr := stdOut[:len(stdOut)-len(".stdout")] + ".stderr"
	return &types.CommandOutputFiles{
		StdErr: stdErr,
		StdOut: stdOut,
	}, nil
}

func ResolveTemplatePath(path string, compileDir types.CompileDir, outputDir types.OutputDir) string {
	path = strings.ReplaceAll(
		path, constants.CompileDirPlaceHolder, string(compileDir),
	)
	path = strings.ReplaceAll(
		path, constants.OutputDirPlaceHolder, string(outputDir),
	)
	return path
}
