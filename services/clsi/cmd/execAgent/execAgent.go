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

package main

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/constants"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

const (
	compileDir = types.CompileDir(constants.CompileDirContainer)
	outputDir  = types.OutputDir(constants.OutputDirContainer)
)

func doExec(ctx context.Context, options *types.ExecAgentRequestOptions, timed *sharedTypes.Timed) (types.ExitCode, error) {
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
	cmd.Env = append(os.Environ(), options.Environment...)

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
	if err2 := ctx.Err(); err2 != nil {
		return -1, err2
	}
	if exitError, isExitError := err.(*exec.ExitError); isExitError {
		return types.ExitCode(exitError.ExitCode()), nil
	}
	return -1, err
}

func do(conn net.Conn, timed *sharedTypes.Timed) (types.ExitCode, string) {
	options := types.ExecAgentRequestOptions{}
	if err := conn.SetDeadline(time.Now().Add(time.Second)); err != nil {
		return -1, "guard slow read: " + err.Error()
	}
	if err := json.NewDecoder(conn).Decode(&options); err != nil {
		return -1, "invalid request"
	}
	deadLine := time.Now().Add(time.Duration(options.ComputeTimeout))
	ctx, done := context.WithDeadline(context.Background(), deadLine)
	defer done()
	t := time.AfterFunc(5*time.Millisecond, func() {
		_ = conn.SetReadDeadline(deadLine)
		_, _ = conn.Read(make([]byte, 1))
		done()
	})
	code, err := doExec(ctx, &options, timed)
	t.Stop()
	switch err {
	case nil:
		return code, ""
	case context.Canceled:
		return code, constants.Cancelled
	case context.DeadlineExceeded:
		return code, constants.TimedOut
	default:
		return code, err.Error()
	}
}

func serve(conn net.Conn) {
	timed := sharedTypes.Timed{}
	code, message := do(conn, &timed)
	msg := &types.ExecAgentResponseBody{
		ExitCode:     code,
		ErrorMessage: message,
		Timed:        timed,
	}
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	_ = json.NewEncoder(conn).Encode(msg)
	_ = conn.Close()
}

func run() (int, error) {
	if len(os.Args) < 2 {
		return 101, &errors.ValidationError{Msg: "missing socket"}
	}
	parts := strings.Split(os.Args[1], "://")
	if len(parts) != 2 {
		return 102, &errors.ValidationError{Msg: "missing proto/spec"}
	}
	proto, address := parts[0], parts[1]
	if proto == "unix" {
		if err := os.Remove(address); err != nil && !os.IsNotExist(err) {
			return 103, errors.Tag(err, "remove unix socket")
		}
	}
	socket, err := net.Listen(proto, address)
	if err != nil {
		return 104, errors.Tag(err, "listen")
	}
	for {
		conn, err2 := socket.Accept()
		if err2 != nil {
			return 105, errors.Tag(err, "accept")
		}
		go serve(conn)
	}
}

func main() {
	code, err := run()
	writeErr := os.WriteFile(
		constants.AgentErrorPathContainer, []byte(err.Error()), 0o600,
	)
	if writeErr != nil {
		code += 100
	}
	os.Exit(code)
}
