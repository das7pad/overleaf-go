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
	"reflect"
	"strings"
	"testing"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func Test_addHeader(t *testing.T) {
	type args struct {
		s        sharedTypes.Snapshot
		learnURL sharedTypes.Snapshot
	}
	tests := []struct {
		name string
		args args
		want sharedTypes.Snapshot
	}{
		{
			name: "works",
			args: args{
				s:        sharedTypes.Snapshot("Hello world!"),
				learnURL: sharedTypes.Snapshot("https://foo.bar/learn"),
			},
			want: sharedTypes.Snapshot(strings.TrimSpace(`
%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
%
% Welcome to Overleaf --- just edit your LaTeX on the left,
% and we'll compile it for you on the right. If you open the
% 'Share' menu, you can invite other users to edit at the same
% time. See https://foo.bar/learn for more info.
% Enjoy!
%
%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%

Hello world!`)),
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			if got := addHeader(tt.args.s, tt.args.learnURL); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("addHeader() = %v, want %v", string(got), string(tt.want))
			}
		})
	}
}
