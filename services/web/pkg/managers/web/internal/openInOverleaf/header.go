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
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

var (
	headerPart1 = sharedTypes.Snippet(`
%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
%
% Welcome to Overleaf --- just edit your LaTeX on the left,
% and we'll compile it for you on the right. If you open the
% 'Share' menu, you can invite other users to edit at the same
% time. See `[1:])
	headerPart2 = sharedTypes.Snippet(` for more info.
% Enjoy!
%
%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%

`)
)

func addHeader(s sharedTypes.Snapshot, learnURL sharedTypes.Snapshot) sharedTypes.Snapshot {
	sum := len(headerPart1) + len(learnURL) + len(headerPart2) + len(s)
	if sum > sharedTypes.MaxDocLength {
		return s
	}
	out := make(sharedTypes.Snapshot, sum)
	n := 0
	n += copy(out[n:], headerPart1)
	n += copy(out[n:], learnURL)
	n += copy(out[n:], headerPart2)
	n += copy(out[n:], s)
	return out
}

func (m *manager) addHeader(s sharedTypes.Snapshot) sharedTypes.Snapshot {
	return addHeader(s, m.learnURL)
}
