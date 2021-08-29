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

package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/constants"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

var containerEnv = os.Environ()

const (
	compileDir = types.CompileDir(constants.CompileDirContainer)
	outputDir  = types.OutputDir(constants.OutputDirContainer)
)

func do(ctx context.Context, options *types.ExecAgentRequestOptions, timed *sharedTypes.Timed) (types.ExitCode, error) {
	args := make([]string, len(options.CommandLine))
	for i, s := range options.CommandLine {
		s = strings.ReplaceAll(
			s, constants.CompileDirPlaceHolder, string(compileDir),
		)
		s = strings.ReplaceAll(
			s, constants.OutputDirPlaceHolder, string(outputDir),
		)
		args[i] = s
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = string(compileDir)
	cmd.Env = append(containerEnv, options.Environment...)

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

	timed.Begin()
	err := cmd.Run()
	timed.End()
	if err == nil {
		return 0, nil
	}
	if exitError, isExitError := err.(*exec.ExitError); isExitError {
		return types.ExitCode(exitError.ExitCode()), nil
	}
	return -1, err
}

func ServeHTTP(w http.ResponseWriter, r *http.Request) {
	options := types.ExecAgentRequestOptions{}
	timed := sharedTypes.Timed{}
	if err := json.NewDecoder(r.Body).Decode(&options); err != nil {
		respond(w, http.StatusBadRequest, -1, timed, "invalid request")
		return
	}

	timeout := time.Duration(options.Timeout)
	ctx, done := context.WithTimeout(r.Context(), timeout)
	defer done()
	code, err := do(ctx, &options, &timed)
	if err == nil {
		respond(w, http.StatusOK, code, timed, "")
	} else if err == context.Canceled {
		respond(w, http.StatusConflict, code, timed, constants.Cancelled)
	} else if err == context.DeadlineExceeded {
		respond(w, http.StatusConflict, code, timed, constants.TimedOut)
	} else {
		respond(w, http.StatusInternalServerError, code, timed, err.Error())
	}
}

func respond(w http.ResponseWriter, status int, code types.ExitCode, timed sharedTypes.Timed, message string) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)
	msg := &types.ExecAgentResponseBody{
		ExitCode:     code,
		ErrorMessage: message,
		Timed:        timed,
	}
	_ = json.NewEncoder(w).Encode(msg)
}

func run() int {
	if len(os.Args) < 2 {
		return 100
	}
	parts := strings.Split(os.Args[1], "://")
	if len(parts) != 2 {
		return 101
	}
	proto, address := parts[0], parts[1]
	if proto == "unix" {
		if err := os.Remove(address); err != nil && !os.IsNotExist(err) {
			return 1
		}
	}
	server := http.Server{Handler: http.HandlerFunc(ServeHTTP)}
	socket, err := net.Listen(proto, address)
	if err != nil {
		return 2
	}
	if err = server.Serve(socket); err != http.ErrServerClosed {
		return 3
	}
	return 0
}

func main() {
	os.Exit(run())
}
