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

package aspellRunner

import (
	"context"
	"strings"

	"github.com/das7pad/overleaf-go/services/spelling/pkg/types"
)

type SuggestionKey struct {
	Language types.SpellCheckLanguage
	Word     string
}

type Suggestions map[SuggestionKey][]string

type Runner interface {
	CheckWords(ctx context.Context, language types.SpellCheckLanguage, words []string) (Suggestions, error)
}

func NewRunner() Runner {
	return &runner{
		wp: newWorkerPool(),
	}
}

type runner struct {
	wp WorkerPool
}

func (r *runner) CheckWords(ctx context.Context, language types.SpellCheckLanguage, words []string) (Suggestions, error) {
	lines, err := r.wp.CheckWords(ctx, language, words)
	if err != nil {
		return nil, err
	}
	return r.parseOutput(language, lines), nil
}

func (r *runner) parseOutput(language types.SpellCheckLanguage, lines []string) Suggestions {
	out := make(Suggestions)
	for _, line := range lines {
		if len(line) < 1 {
			continue
		}
		parts := strings.SplitN(line, " ", 5)
		hasSuggestions := len(parts) == 5 &&
			parts[0] == "&" &&
			strings.HasSuffix(parts[3], ":")
		if hasSuggestions {
			suggestions := strings.Split(parts[4], ", ")

			key := SuggestionKey{Language: language, Word: parts[1]}
			out[key] = suggestions
			continue
		}

		hasNoSuggestions := len(parts) == 3 && parts[0] == "#"
		if hasNoSuggestions {
			key := SuggestionKey{Language: language, Word: parts[1]}
			out[key] = nil
		}
	}
	return out
}
