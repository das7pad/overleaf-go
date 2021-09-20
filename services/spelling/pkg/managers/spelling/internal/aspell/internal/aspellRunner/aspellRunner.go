// Golang port of Overleaf
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

package aspellRunner

import (
	"context"
	"strings"
)

type SuggestionKey struct {
	Language string
	Word     string
}

type Suggestions map[SuggestionKey][]string

type Runner interface {
	CheckWords(
		ctx context.Context,
		language string,
		words []string,
	) (Suggestions, error)
}

func NewRunner() Runner {
	return &runner{
		wp: newWorkerPool(),
	}
}

type runner struct {
	wp WorkerPool
}

func (r *runner) CheckWords(
	ctx context.Context,
	language string,
	words []string,
) (Suggestions, error) {
	lines, err := r.wp.CheckWords(ctx, language, words)
	if err != nil {
		return nil, err
	}
	return r.parseOutput(language, lines), nil
}

var NoSuggestions []string

func (r *runner) parseOutput(language string, lines []string) Suggestions {
	out := make(Suggestions)
	for _, line := range lines {
		if len(line) < 1 {
			continue
		}
		hasSuggestions := line[0] == '&' &&
			strings.ContainsRune(line, ' ') &&
			strings.ContainsRune(line, ':') &&
			len(line) > (strings.IndexRune(line, ':')+2)
		if hasSuggestions {
			parts := strings.SplitN(line, " ", 3)
			word := parts[1]

			suggestions := strings.Split(
				line[strings.IndexRune(line, ':')+2:],
				", ",
			)

			key := SuggestionKey{language, word}
			out[key] = suggestions
			continue
		}

		hasNoSuggestions := line[0] == '#' &&
			strings.ContainsRune(line, ' ')
		if hasNoSuggestions {
			parts := strings.SplitN(line, " ", 2)
			word := parts[1]

			key := SuggestionKey{language, word}
			out[key] = NoSuggestions
		}
	}
	return out
}
