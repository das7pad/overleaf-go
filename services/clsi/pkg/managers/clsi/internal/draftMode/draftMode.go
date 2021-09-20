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

package draftMode

import (
	"regexp"
	"strings"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type Manager interface {
	InjectDraftMode(rootDoc *types.Resource) error
}

func New() Manager {
	return &manager{}
}

type manager struct {
}

const (
	draftModePresent = "\\documentclass[draft"
)

var (
	withOptions            = regexp.MustCompile("\\\\documentclass\\[")
	withOptionsReplacement = []byte("\\documentclass[draft,")

	noOptions            = regexp.MustCompile("\\\\documentclass{")
	noOptionsReplacement = []byte("\\documentclass[draft]{")
)

func (m *manager) InjectDraftMode(rootDoc *types.Resource) error {
	blob := string(*rootDoc.Content)
	if strings.Contains(blob, draftModePresent) {
		return nil
	}

	content := sharedTypes.Snapshot(string(
		noOptions.ReplaceAll(
			withOptions.ReplaceAll(
				[]byte(blob),
				withOptionsReplacement,
			),
			noOptionsReplacement,
		),
	))
	rootDoc.Content = &content
	return nil
}
