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
	) (*types.CommandOutputFiles, error)
}

func New(options *types.Options) LatexRunner {
	return &latexRunner{options: options}
}

type latexRunner struct {
	options *types.Options
}

var (
	compilerFlag = map[types.Compiler]string{
		types.Latex:    "-pdfdvi",
		types.LuaLatex: "-lualatex",
		types.PDFLatex: "-pdf",
		types.XeLatex:  "-xelatex",
	}

	preProcessedFileTypes = []sharedTypes.FileType{
		"md",
		"Rtx",
		"Rmd",
	}
)

func (r *latexRunner) Run(ctx context.Context, run commandRunner.NamespacedRun, namespace types.Namespace, request *types.CompileRequest, response *types.CompileResponse) (*types.CommandOutputFiles, error) {
	cmd, err := r.composeCommandOptions(namespace, request, response)
	if err != nil {
		return nil, err
	}

	code, err := run(ctx, cmd)
	status := response.Status
	if err == nil {
		// NOTE: This is mimicking the NodeJS implementation.
		if code == 1 {
			status = constants.Failure
		} else {
			status = constants.Success
		}
	} else if err == context.DeadlineExceeded {
		status = constants.TimedOut
	} else if err == context.Canceled {
		status = constants.Terminated
	} else {
		cmd.CommandOutputFiles.Cleanup(
			r.options.CompileBaseDir.CompileDir(namespace),
		)
		return nil, err
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

	return &cmd.CommandOutputFiles, nil
}

func (r *latexRunner) composeCommandOptions(namespace types.Namespace, request *types.CompileRequest, response *types.CompileResponse) (*types.CommandOptions, error) {
	files, err := commandRunner.CreateCommandOutput(
		r.options.CompileBaseDir.CompileDir(namespace),
	)
	if err != nil {
		return nil, err
	}

	mainFile := string(request.RootResourcePath)
	fileType := sharedTypes.FileName(mainFile).Type()
	for _, preProcessedFileType := range preProcessedFileTypes {
		if fileType == preProcessedFileType {
			mainFile = mainFile[:len(mainFile)-len(fileType)] + "tex"
			break
		}
	}
	cmd := types.CommandLine{
		"latexmk",
		"-cd",
		"-f",
		"-jobname=output",
		"-auxdir=" + constants.CompileDirPlaceHolder,
		"-outdir=" + constants.CompileDirPlaceHolder,
		"-synctex=1",
		"-interaction=batchmode",
		compilerFlag[request.Options.Compiler],
		constants.CompileDirPlaceHolder + "/" + mainFile,
	}

	env := r.options.LatexBaseEnv

	isTexFile := sharedTypes.FileName(request.RootResourcePath).Type() == "tex"
	checkMode := request.Options.Check
	if checkMode != types.NoCheck && isTexFile {
		env = append(
			env,
			"CHKTEX_OPTIONS=-nall -e9 -e10 -w15 -w16",
			"CHKTEX_ULIMIT_OPTIONS=-t 5 -v 64000",
		)
		if checkMode == types.ErrorCheck {
			env = append(env, "CHKTEX_EXIT_ON_ERROR=1")
		} else if checkMode == types.ValidateCheck {
			env = append(env, "CHKTEX_VALIDATE=1")
		}
	}

	return &types.CommandOptions{
		CommandLine:        cmd,
		Environment:        env,
		ImageName:          request.Options.ImageName,
		Timeout:            request.Options.Timeout,
		CompileGroup:       request.Options.CompileGroup,
		CommandOutputFiles: *files,
		Timed:              &response.Timings.Compile,
	}, nil
}
