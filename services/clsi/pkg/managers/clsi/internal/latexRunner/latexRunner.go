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

package latexRunner

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/constants"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/commandRunner"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type LatexRunner interface {
	Run(
		ctx context.Context,
		run commandRunner.NamespacedRun,
		namespace types.Namespace,
		request *types.CompileRequest,
		response *types.CompileResponse,
	) error
}

func New(options *types.Options) LatexRunner {
	return &latexRunner{options: options}
}

type latexRunner struct {
	options *types.Options
}

var preProcessedFileTypes = []sharedTypes.FileType{
	"md",
	"Rtx",
	"Rmd",
}

func (r *latexRunner) Run(ctx context.Context, run commandRunner.NamespacedRun, namespace types.Namespace, request *types.CompileRequest, response *types.CompileResponse) error {
	cmd := r.composeCommandOptions(request, response)

	code, err := run(ctx, cmd)
	var status types.CompileStatus
	switch err {
	case nil:
		// NOTE: This is mimicking the NodeJS implementation.
		if code == 1 {
			status = constants.Failure
		} else {
			status = constants.Success
		}
	case context.DeadlineExceeded:
		status = constants.TimedOut
	case context.Canceled:
		status = constants.Terminated
	default:
		cmd.CommandOutputFiles.Cleanup(
			r.options.CompileBaseDir.CompileDir(namespace),
		)
		return err
	}
	if request.Options.Check == types.ValidateCheck {
		status = constants.ValidationPass
		if code != 0 {
			status = constants.ValidationFail
		}
	} else if request.Options.Check == types.ErrorCheck {
		if code != 0 {
			status = constants.ValidationFail
		}
	}
	response.Status = status

	return nil
}

func (r *latexRunner) composeCommandOptions(request *types.CompileRequest, response *types.CompileResponse) *types.CommandOptions {
	mainFile := string(request.RootResourcePath)
	fileType := sharedTypes.PathName(mainFile).Type()
	for _, preProcessedFileType := range preProcessedFileTypes {
		if fileType == preProcessedFileType {
			mainFile = mainFile[:len(mainFile)-len(fileType)] + "tex"
			break
		}
	}
	//goland:noinspection SpellCheckingInspection
	cmd := types.CommandLine{
		"latexmk",
		"-cd",
		"-f",
		"-jobname=output",
		"-auxdir=" + constants.CompileDirPlaceHolder,
		"-outdir=" + constants.CompileDirPlaceHolder,
		"-synctex=1",
		"-interaction=batchmode",
		request.Options.Compiler.LaTeXmkFlag(),
		constants.CompileDirPlaceHolder + "/" + mainFile,
	}

	env := r.options.LatexBaseEnv

	isTexFile := request.RootResourcePath.Type() == "tex"
	checkMode := request.Options.Check
	if checkMode != types.NoCheck && isTexFile {
		//goland:noinspection SpellCheckingInspection
		env = append(
			env,
			"CHKTEX_OPTIONS=-nall -e9 -e10 -w15 -w16",
			"CHKTEX_ULIMIT_OPTIONS=-t 5 -v 64000",
		)
		if checkMode == types.ErrorCheck {
			//goland:noinspection SpellCheckingInspection
			env = append(env, "CHKTEX_EXIT_ON_ERROR=1")
		} else if checkMode == types.ValidateCheck {
			//goland:noinspection SpellCheckingInspection
			env = append(env, "CHKTEX_VALIDATE=1")
		}
	}

	files := types.CommandOutputFiles{
		StdErr: "output.stderr",
		StdOut: "output.stdout",
	}
	return &types.CommandOptions{
		CommandLine:        cmd,
		Environment:        env,
		ImageName:          request.Options.ImageName,
		ComputeTimeout:     request.Options.Timeout,
		CompileGroup:       request.Options.CompileGroup,
		CommandOutputFiles: files,
		Timed:              &response.Timings.Compile,
	}
}
