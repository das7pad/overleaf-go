// Golang port of Overleaf
// Copyright (C) 2021-2022 Jakob Ackermann <das7pad@outlook.com>
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
	"os/exec"
	"strings"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

func newLocalRunner(options *types.Options) (Runner, error) {
	return &localRunner{paths: options.Paths}, nil
}

type localRunner struct {
	paths types.Paths
}

func (l *localRunner) Stop(_ types.Namespace) error {
	// We do not keep any process table. Nothing to stop.
	return nil
}

func (l *localRunner) Setup(_ context.Context, _ types.Namespace, _ sharedTypes.ImageName) (*time.Time, error) {
	validUntilTomorrow := time.Now().Add(time.Hour * 24)
	return &validUntilTomorrow, nil
}

func (l *localRunner) Run(ctx context.Context, namespace types.Namespace, options *types.CommandOptions) (types.ExitCode, error) {
	compileDir := l.paths.CompileBaseDir.CompileDir(namespace)
	outputDir := l.paths.OutputBaseDir.OutputDir(namespace)
	args := make([]string, len(options.CommandLine))
	for i, s := range options.CommandLine {
		args[i] = ResolveTemplatePath(s, compileDir, outputDir)
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = string(compileDir)
	cmd.Env = options.Environment

	if options.CommandOutputFiles.StdErr != "" {
		stdErr, err := os.Create(compileDir.Join(options.StdErr))
		if err != nil {
			return -1, err
		}
		cmd.Stderr = stdErr
	}
	if options.CommandOutputFiles.StdOut != "" {
		stdOut, err := os.Create(compileDir.Join(options.StdOut))
		if err != nil {
			return -1, err
		}
		cmd.Stdout = stdOut
	}

	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	if exitError, isExitError := err.(*exec.ExitError); isExitError {
		return types.ExitCode(exitError.ExitCode()), nil
	}
	return -1, err
}

func (l *localRunner) Resolve(path string, namespace types.Namespace) (sharedTypes.PathName, error) {
	compileDir := string(l.paths.CompileBaseDir.CompileDir(namespace))
	if strings.HasPrefix(path, compileDir+"/") {
		return sharedTypes.PathName(path[len(compileDir)+1:]), nil
	}
	outputDir := string(l.paths.OutputBaseDir.OutputDir(namespace))
	if strings.HasPrefix(path, outputDir+"/") {
		return sharedTypes.PathName(path[len(outputDir)+1:]), nil
	}
	return "", errors.New("unknown base: " + path)
}
