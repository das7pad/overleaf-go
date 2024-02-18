// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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

package project

import (
	"testing"
)

func TestNames_MakeUnique(t *testing.T) {
	type args struct {
		source Name
	}
	tests := []struct {
		name  string
		names Names
		args  args
		want  Name
	}{
		{
			name:  "no names",
			names: Names{},
			args:  args{source: "foo"},
			want:  "foo",
		},
		{
			name:  "one name",
			names: Names{"foo"},
			args:  args{source: "foo"},
			want:  "foo (1)",
		},
		{
			name:  "trim suffix",
			names: Names{},
			args:  args{source: "foo (1)"},
			want:  "foo",
		},
		{
			name:  "unique source",
			names: Names{"bar"},
			args:  args{source: "foo"},
			want:  "foo",
		},
		{
			name:  "unique after trim suffix",
			names: Names{"foo (1)"},
			args:  args{source: "foo (1)"},
			want:  "foo",
		},
		{
			name:  "bump suffix",
			names: Names{"foo", "foo (1)"},
			args:  args{source: "foo (1)"},
			want:  "foo (2)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.names.MakeUnique(tt.args.source); got != tt.want {
				t.Errorf("MakeUnique() = %v, want %v", got, tt.want)
			}
		})
	}
}
