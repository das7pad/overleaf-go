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

package aspell

import (
	"context"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/das7pad/overleaf-go/services/spelling/pkg/managers/spelling/internal/aspell/internal/aspellRunner"
	"github.com/das7pad/overleaf-go/services/spelling/pkg/types"
)

type Manager interface {
	CheckWords(
		ctx context.Context,
		language types.SpellCheckLanguage,
		words []string,
	) ([]types.Misspelling, error)
}

func New(lruSize int) (Manager, error) {
	cache, err := lru.New[aspellRunner.SuggestionKey, []string](lruSize)
	if err != nil {
		return nil, err
	}
	return &manager{
		cache:  cache,
		runner: aspellRunner.NewRunner(),
	}, nil
}

const (
	RequestLimit = 10000
)

type manager struct {
	cache  *lru.Cache[aspellRunner.SuggestionKey, []string]
	runner aspellRunner.Runner
}

func (m *manager) CheckWords(
	ctx context.Context,
	language types.SpellCheckLanguage,
	words []string,
) ([]types.Misspelling, error) {
	if err := language.Validate(); err != nil {
		return nil, err
	}
	if len(words) > RequestLimit {
		words = words[:RequestLimit]
	}

	suggestions := make(aspellRunner.Suggestions)
	runOnWordsDedupe := make(map[string]bool, 0)
	for _, word := range words {
		if runOnWordsDedupe[word] {
			continue
		}
		key := aspellRunner.SuggestionKey{Language: language, Word: word}
		if _, exists := suggestions[key]; exists {
			continue
		}
		// ^ do not hit the cache for duplicate words

		if items, exists := m.cache.Get(key); exists {
			suggestions[key] = items
		} else {
			runOnWordsDedupe[word] = true
		}
	}
	if len(runOnWordsDedupe) > 0 {
		runOnWords := make([]string, 0, len(runOnWordsDedupe))
		for word := range runOnWordsDedupe {
			runOnWords = append(runOnWords, word)
		}
		newSuggestions, err := m.runner.CheckWords(ctx, language, runOnWords)
		if err != nil {
			return nil, err
		}
		for _, word := range runOnWords {
			key := aspellRunner.SuggestionKey{Language: language, Word: word}
			items := newSuggestions[key]
			suggestions[key] = items
			m.cache.Add(key, items)
		}
	}
	misspellings := make([]types.Misspelling, 0)
	for idx, word := range words {
		key := aspellRunner.SuggestionKey{Language: language, Word: word}
		items := suggestions[key]
		if items == nil {
			// not misspelled
			continue
		}
		misspellings = append(misspellings, types.Misspelling{
			Index:       idx,
			Suggestions: items,
		})
	}
	return misspellings, nil
}
