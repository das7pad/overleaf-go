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

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/spelling/pkg/managers/spelling/internal/aspell/internal/aspellRunner"
	"github.com/das7pad/overleaf-go/services/spelling/pkg/types"
)

type Manager interface {
	CheckWords(ctx context.Context, language types.SpellCheckLanguage, words []string) ([]types.Misspelling, error)
}

func New(lruSize int) (Manager, error) {
	caches := make(
		map[types.SpellCheckLanguage]*lru.Cache[string, []string],
		len(types.AllowedLanguages),
	)
	for _, language := range types.AllowedLanguages {
		cache, err := lru.New[string, []string](lruSize)
		if err != nil {
			return nil, err
		}
		caches[language] = cache
	}

	return &manager{
		caches: caches,
		wp:     aspellRunner.NewWorkerPool(),
	}, nil
}

const (
	RequestLimit = 10_000
)

type manager struct {
	caches map[types.SpellCheckLanguage]*lru.Cache[string, []string]
	wp     aspellRunner.WorkerPool
}

func (m *manager) CheckWords(ctx context.Context, language types.SpellCheckLanguage, words []string) ([]types.Misspelling, error) {
	if err := language.Validate(); err != nil {
		return nil, err
	}
	if len(words) > RequestLimit {
		words = words[:RequestLimit]
	}
	cache := m.caches[language]

	suggestions := make(aspellRunner.Suggestions, len(words))
	cacheStatus := make(map[string]bool, len(words))
	for _, word := range words {
		if _, found := cacheStatus[word]; found {
			return nil, &errors.ValidationError{Msg: "duplicate word"}
		}
		items, exists := cache.Get(word)
		cacheStatus[word] = exists
		if exists {
			suggestions[word] = items
		}
	}
	if missing := len(cacheStatus) - len(suggestions); missing > 0 {
		runOnWords := make([]string, 0, missing)
		for word, exists := range cacheStatus {
			if !exists {
				runOnWords = append(runOnWords, word)
			}
		}
		newSuggestions, err := m.wp.CheckWords(ctx, language, runOnWords)
		if err != nil {
			return nil, err
		}
		for _, word := range runOnWords {
			items := newSuggestions[word]
			suggestions[word] = items
			cache.Add(word, items)
		}
	}
	misspellings := make([]types.Misspelling, 0)
	for idx, word := range words {
		items := suggestions[word]
		if items == nil {
			continue // not misspelled
		}
		misspellings = append(misspellings, types.Misspelling{
			Index:       idx,
			Suggestions: items,
		})
	}
	return misspellings, nil
}
