// Golang port of Overleaf
// Copyright (C) 2023 Jakob Ackermann <das7pad@outlook.com>
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

package less

import (
	"testing"
)

func Test_evalMath(t *testing.T) {
	type args struct {
		s tokens
	}
	tests := []struct {
		name string
		args args
		want tokens
	}{
		{
			name: "single operation space",
			args: args{
				s: tokenize("1 + 1", ""),
			},
			want: tokenize("2", ""),
		},
		{
			name: "single operation no space",
			args: args{
				s: tokenize("1+1", ""),
			},
			want: tokenize("2", ""),
		},
		{
			name: "single operation same unit",
			args: args{
				s: tokenize("1px + 1px", ""),
			},
			want: tokenize("2px", ""),
		},
		{
			name: "single operation unit first",
			args: args{
				s: tokenize("2px * 2", ""),
			},
			want: tokenize("4px", ""),
		},
		{
			name: "single operation unit second",
			args: args{
				s: tokenize("2 * 2px", ""),
			},
			want: tokenize("4px", ""),
		},
		{
			name: "single operation div unit first",
			args: args{
				s: tokenize("2px / 2", ""),
			},
			want: tokenize("1px", ""),
		},
		{
			name: "single operation div unit both",
			args: args{
				s: tokenize("6px / 3px", ""),
			},
			want: tokenize("2", ""),
		},
		{
			name: "single operation parens",
			args: args{
				s: tokenize("(1+1)", ""),
			},
			want: tokenize("2", ""),
		},
		{
			name: "single operation parens minus",
			args: args{
				s: tokenize("-(1+1)", ""),
			},
			want: tokenize("-2", ""),
		},
		{
			name: "two operations plus",
			args: args{
				s: tokenize("1+1+2", ""),
			},
			want: tokenize("4", ""),
		},
		{
			name: "two operations minus",
			args: args{
				s: tokenize("1-1-2", ""),
			},
			want: tokenize("-2", ""),
		},
		{
			name: "two operations minus plus",
			args: args{
				s: tokenize("1-1+2", ""),
			},
			want: tokenize("2", ""),
		},
		{
			name: "two operations plus times",
			args: args{
				s: tokenize("1+1*2", ""),
			},
			want: tokenize("3", ""),
		},
		{
			name: "two operations minus times",
			args: args{
				s: tokenize("1-1*2", ""),
			},
			want: tokenize("-1", ""),
		},
		{
			name: "two operations div times",
			args: args{
				s: tokenize("4/2*3", ""),
			},
			want: tokenize("6", ""),
		},
		{
			name: "two operations times div",
			args: args{
				s: tokenize("2*3/6", ""),
			},
			want: tokenize("1", ""),
		},
		{
			name: "two operations parens plus",
			args: args{
				s: tokenize("(1+1)+2", ""),
			},
			want: tokenize("4", ""),
		},
		{
			name: "two operations parens minus",
			args: args{
				s: tokenize("(1-1)-2", ""),
			},
			want: tokenize("-2", ""),
		},
		{
			name: "two operations parens times",
			args: args{
				s: tokenize("(1+1)*2", ""),
			},
			want: tokenize("4", ""),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := evalMath(tt.args.s); got.String() != tt.want.String() {
				t.Errorf("evalMath() = %v, want %v", got, tt.want)
			}
		})
	}
}
