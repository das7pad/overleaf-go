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

package wordCounter

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/constants"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/commandRunner"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type Counter interface {
	Count(
		ctx context.Context,
		run commandRunner.NamespacedRun,
		namespace types.Namespace,
		request *types.WordCountRequest,
		words *types.Words,
	) error
}

func New(options *types.Options) Counter {
	return &counter{
		compileBaseDir: options.CompileBaseDir,
	}
}

type counter struct {
	compileBaseDir types.CompileBaseDir
}

const timeout = 60 * time.Second

func (c *counter) Count(ctx context.Context, run commandRunner.NamespacedRun, namespace types.Namespace, request *types.WordCountRequest, words *types.Words) error {
	compileDir := c.compileBaseDir.CompileDir(namespace)
	doc := constants.CompileDirPlaceHolder + "/" + string(request.FileName)

	files, err := commandRunner.CreateCommandOutput(compileDir)
	if err != nil {
		return err
	}
	defer files.Cleanup(compileDir)

	//goland:noinspection SpellCheckingInspection
	cmd := types.CommandLine{
		"texcount",
		"-inc",
		"-nocol",
		"-nocodes",
		"-nosub",
		"-nosum",
		"-out-stderr",
		doc,
	}
	options := &types.CommandOptions{
		CommandLine:        cmd,
		ImageName:          request.ImageName,
		ComputeTimeout:     sharedTypes.ComputeTimeout(timeout),
		CompileGroup:       request.CompileGroup,
		CommandOutputFiles: *files,
	}
	code, err := run(ctx, options)
	if err != nil {
		return err
	}
	if code != 0 {
		return errors.New("non success from texcount")
	}

	blob, err := os.ReadFile(compileDir.Join(files.StdErr))
	if err != nil {
		return err
	}

	for _, line := range strings.Split(string(blob), "\n") {
		if line == "" ||
			line[0] == ' ' ||
			strings.HasPrefix(line, "File:") {
			continue
		}
		if strings.HasPrefix(line, "Encoding: ") {
			words.Encode = line[len("Encoding: "):]
			continue
		}
		if strings.HasPrefix(line, "!!!") {
			words.Messages += line + "\n"
			continue
		}
		if strings.HasPrefix(line, "(errors:") {
			// (errors:123)
			raw := line[len("(errors:") : len(line)-1]
			n, nonInt := strconv.ParseInt(raw, 10, 64)
			if nonInt != nil {
				continue
			}
			words.Errors = n
		}

		parts := strings.SplitN(line, ": ", 2)
		if len(parts) != 2 {
			continue
		}
		n, nonInt := strconv.ParseInt(parts[1], 10, 64)
		if nonInt != nil {
			continue
		}
		switch parts[0] {
		case "Words in text":
			words.TextWords = n
		case "Words in headers":
			words.HeadWords = n
		case "Words outside text (captions, etc.)":
			words.Outside = n
		case "Number of headers":
			words.Headers = n
		case "Number of floats/tables/figures":
			words.Elements = n
		case "Number of math inlines":
			words.MathInline = n
		case "Number of math displayed":
			words.MathDisplay = n
		}
	}
	return nil
}
