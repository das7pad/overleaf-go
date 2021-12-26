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

package openInOverleaf

import (
	"strings"

	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

//goland:noinspection SpellCheckingInspection
var (
	documentClassPart1 = sharedTypes.Snippet(`
\documentclass[12pt]{article}
\usepackage[english]{babel}
\usepackage[utf8x]{inputenc}
\usepackage{amsmath}
\usepackage{tikz}
\begin{document}
\title{`[1:])
	documentClassPart2 = sharedTypes.Snippet(`
}

`[1:])
	documentClassPart3 = sharedTypes.Snippet(`

\end{document}`)
)

func addDocumentClass(s sharedTypes.Snapshot, name project.Name) sharedTypes.Snapshot {
	limit := 10 * 1024
	if len(s) < limit {
		limit = len(s)
	}
	if strings.Contains(string(s[:limit]), "\\documentclass") {
		return s
	}
	title := sharedTypes.Snippet(name)
	sum := len(documentClassPart1) +
		len(title) +
		len(documentClassPart2) +
		len(s) +
		len(documentClassPart3)
	if sum > sharedTypes.MaxDocLength {
		return s
	}
	out := make(sharedTypes.Snapshot, sum)
	n := 0
	n += copy(out[n:], documentClassPart1)
	n += copy(out[n:], title)
	n += copy(out[n:], documentClassPart2)
	n += copy(out[n:], s)
	n += copy(out[n:], documentClassPart3)
	return out
}
