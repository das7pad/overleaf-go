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

package syncTex

import (
	"context"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/commandRunner"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type Manager interface {
	FromCode(
		ctx context.Context,
		run commandRunner.NamespacedRun,
		namespace types.Namespace,
		request *types.SyncFromCodeRequest,
		positions *types.PDFPositions,
	) error

	FromPDF(
		ctx context.Context,
		run commandRunner.NamespacedRun,
		namespace types.Namespace,
		request *types.SyncFromPDFRequest,
		positions *types.CodePositions,
	) error
}

func New(options *types.Options, runner commandRunner.Runner) Manager {
	return &manager{
		options: options,
		runner:  runner,
	}
}

type manager struct {
	options *types.Options
	runner  commandRunner.Runner
}

func (m *manager) FromCode(ctx context.Context, run commandRunner.NamespacedRun, namespace types.Namespace, request *types.SyncFromCodeRequest, positions *types.PDFPositions) error {
	lines, err := m.runSyncTex(ctx, run, namespace, request)
	if err != nil {
		return err
	}

	*positions = types.PDFPositions{}
	p := &types.PDFPosition{}
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		label, raw := parts[0], parts[1]
		switch label {
		case "Output":
			p = &types.PDFPosition{}
			*positions = append(*positions, p)

		case "Page":
			i, parseErr := strconv.ParseInt(raw, 10, 64)
			if parseErr != nil {
				return parseErr
			}
			p.Page = types.Page(i)
		case "h", "v", "H", "W":
			f, parseErr := strconv.ParseFloat(raw, 64)
			if parseErr != nil {
				return parseErr
			}
			// Mimic legacy results: Truncate decimal places to two.
			f = math.Round(f*100) / 100
			switch label {
			case "h":
				p.Horizontal = types.Horizontal(f)
			case "v":
				p.Vertical = types.Vertical(f)
			case "H":
				p.Height = types.Height(f)
			case "W":
				p.Width = types.Width(f)
			}
		}
	}
	return nil
}

func (m *manager) FromPDF(ctx context.Context, run commandRunner.NamespacedRun, namespace types.Namespace, request *types.SyncFromPDFRequest, positions *types.CodePositions) error {
	lines, err := m.runSyncTex(ctx, run, namespace, request)
	if err != nil {
		return err
	}

	*positions = types.CodePositions{}
	p := &types.CodePosition{}
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		label, raw := parts[0], parts[1]
		switch label {
		case "Output":
			p = &types.CodePosition{}
			*positions = append(*positions, p)

		case "Input":
			f, resolveErr := m.runner.Resolve(raw, namespace)
			if resolveErr != nil {
				return resolveErr
			}
			p.FileName = f
		case "Line", "Column":
			i, parseErr := strconv.ParseInt(raw, 10, 64)
			if parseErr != nil {
				return parseErr
			}
			switch label {
			case "Line":
				p.Line = types.Line(i)
			case "Column":
				p.Column = types.Column(i)
			}
		}
	}
	return nil
}

const timeout = 60 * time.Second

func (m *manager) runSyncTex(ctx context.Context, run commandRunner.NamespacedRun, namespace types.Namespace, request types.SyncTexRequestCommon) ([]string, error) {
	compileDir := m.options.CompileBaseDir.CompileDir(namespace)
	outputDir := m.options.OutputBaseDir.OutputDir(namespace)
	syncTexOptions := request.Options()

	p := commandRunner.ResolveTemplatePath(
		syncTexOptions.OutputSyncTexGzPath(),
		compileDir,
		outputDir,
	)
	if _, statErr := os.Lstat(p); os.IsNotExist(statErr) {
		return nil, &errors.MissingOutputFileError{
			Msg: "cannot sync code/pdf: output.synctex.gz missing",
		}
	}

	files, err := commandRunner.CreateCommandOutput(compileDir)
	if err != nil {
		return nil, err
	}
	defer files.Cleanup(compileDir)

	cmd := &types.CommandOptions{
		CommandLine:        request.CommandLine(),
		ImageName:          syncTexOptions.ImageName,
		Timeout:            types.Timeout(timeout),
		CompileGroup:       syncTexOptions.CompileGroup,
		CommandOutputFiles: *files,
	}
	code, err := run(ctx, cmd)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, errors.New("non success from synctex")
	}

	blob, err := os.ReadFile(compileDir.Join(files.StdOut))
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return strings.Split(string(blob), "\n"), nil
}