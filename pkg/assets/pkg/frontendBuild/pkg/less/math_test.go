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
			name: "static no space",
			args: args{
				s: tokenize("1", 0),
			},
			want: tokenize("1", 0),
		},
		{
			name: "static leading space",
			args: args{
				s: tokenize("  1", 0),
			},
			want: tokenize("  1", 0),
		},
		{
			name: "static negative no space",
			args: args{
				s: tokenize("-1", 0),
			},
			want: tokenize("-1", 0),
		},
		{
			name: "static negative leading space",
			args: args{
				s: tokenize("  -1", 0),
			},
			want: tokenize("  -1", 0),
		},
		{
			name: "single operation leading space",
			args: args{
				s: tokenize("  1+1", 0),
			},
			want: tokenize("2", 0),
		},
		{
			name: "single operation space",
			args: args{
				s: tokenize("1 + 1", 0),
			},
			want: tokenize("2", 0),
		},
		{
			name: "single operation no space",
			args: args{
				s: tokenize("1+1", 0),
			},
			want: tokenize("2", 0),
		},
		{
			name: "single operation same unit",
			args: args{
				s: tokenize("1px + 1px", 0),
			},
			want: tokenize("2px", 0),
		},
		{
			name: "single operation unit first",
			args: args{
				s: tokenize("2px * 2", 0),
			},
			want: tokenize("4px", 0),
		},
		{
			name: "single operation unit second",
			args: args{
				s: tokenize("2 * 2px", 0),
			},
			want: tokenize("4px", 0),
		},
		{
			name: "single operation div unit first",
			args: args{
				s: tokenize("2px / 2", 0),
			},
			want: tokenize("1px", 0),
		},
		{
			name: "single operation div unit both",
			args: args{
				s: tokenize("6px / 3px", 0),
			},
			want: tokenize("2", 0),
		},
		{
			name: "single operation parens",
			args: args{
				s: tokenize("(1+1)", 0),
			},
			want: tokenize("2", 0),
		},
		{
			name: "single operation parens extra space",
			args: args{
				s: tokenize("(1+1 )", 0),
			},
			want: tokenize("2", 0),
		},
		{
			name: "single operation parens minus",
			args: args{
				s: tokenize("-(1+1)", 0),
			},
			want: tokenize("-2", 0),
		},
		{
			name: "two operations plus",
			args: args{
				s: tokenize("1+1+2", 0),
			},
			want: tokenize("4", 0),
		},
		{
			name: "two operations minus",
			args: args{
				s: tokenize("1-1-2", 0),
			},
			want: tokenize("-2", 0),
		},
		{
			name: "two operations minus plus",
			args: args{
				s: tokenize("1-1+2", 0),
			},
			want: tokenize("2", 0),
		},
		{
			name: "two operations plus times",
			args: args{
				s: tokenize("1+1*2", 0),
			},
			want: tokenize("3", 0),
		},
		{
			name: "two operations minus times",
			args: args{
				s: tokenize("1-1*2", 0),
			},
			want: tokenize("-1", 0),
		},
		{
			name: "two operations div times",
			args: args{
				s: tokenize("4/2*3", 0),
			},
			want: tokenize("6", 0),
		},
		{
			name: "two operations times div",
			args: args{
				s: tokenize("2*3/6", 0),
			},
			want: tokenize("1", 0),
		},
		{
			name: "two operations parens plus",
			args: args{
				s: tokenize("(1+1)+2", 0),
			},
			want: tokenize("4", 0),
		},
		{
			name: "two operations parens minus",
			args: args{
				s: tokenize("(1-1)-2", 0),
			},
			want: tokenize("-2", 0),
		},
		{
			name: "two operations parens times",
			args: args{
				s: tokenize("(1+1)*2", 0),
			},
			want: tokenize("4", 0),
		},
		{
			name: "unit one arg",
			args: args{
				s: tokenize("unit(2rem)", 0),
			},
			want: tokenize("2", 0),
		},
		{
			name: "unit two args",
			args: args{
				s: tokenize("unit(2, rem)", 0),
			},
			want: tokenize("2rem", 0),
		},
		{
			name: "ceil",
			args: args{
				s: tokenize("ceil(1.1)", 0),
			},
			want: tokenize("2", 0),
		},
		{
			name: "ceil",
			args: args{
				s: tokenize("ceil(1.4)", 0),
			},
			want: tokenize("2", 0),
		},
		{
			name: "floor",
			args: args{
				s: tokenize("floor(1.5)", 0),
			},
			want: tokenize("1", 0),
		},
		{
			name: "round",
			args: args{
				s: tokenize("round(1.5)", 0),
			},
			want: tokenize("2", 0),
		},
		{
			name: "percentage",
			args: args{
				s: tokenize("percentage(0.5555)", 0),
			},
			want: tokenize("55.55%", 0),
		},
		{
			name: "sqrt",
			args: args{
				s: tokenize("sqrt(4)", 0),
			},
			want: tokenize("2", 0),
		},
		{
			name: "abs",
			args: args{
				s: tokenize("abs(-1.5)", 0),
			},
			want: tokenize("1.5", 0),
		},
		{
			name: "sin",
			args: args{
				s: tokenize("sin(1)", 0),
			},
			want: tokenize("0.8414709848078965", 0),
		},
		{
			name: "cos",
			args: args{
				s: tokenize("cos(1)", 0),
			},
			want: tokenize("0.5403023058681398", 0),
		},
		{
			name: "tan",
			args: args{
				s: tokenize("tan(1)", 0),
			},
			want: tokenize("1.557407724654902", 0),
		},
		{
			name: "asin",
			args: args{
				s: tokenize("asin(1)", 0),
			},
			want: tokenize("1.5707963267948966", 0),
		},
		{
			name: "acos",
			args: args{
				s: tokenize("acos(0.5)", 0),
			},
			want: tokenize("1.0471975511965976", 0),
		},
		{
			name: "atan",
			args: args{
				s: tokenize("atan(1)", 0),
			},
			want: tokenize("0.7853981633974483", 0),
		},
		{
			name: "pow",
			args: args{
				s: tokenize("pow(2, 2)", 0),
			},
			want: tokenize("4", 0),
		},
		{
			name: "mod",
			args: args{
				s: tokenize("mod(7, 3)", 0),
			},
			want: tokenize("1", 0),
		},
		{
			name: "mod unit",
			args: args{
				s: tokenize("mod(7px, 3)", 0),
			},
			want: tokenize("1px", 0),
		},
		{
			name: "min",
			args: args{
				s: tokenize("min(3px, 7px, 2.1px)", 0),
			},
			want: tokenize("2.1px", 0),
		},
		{
			name: "max",
			args: args{
				s: tokenize("max(3px, 7px, 2.1px)", 0),
			},
			want: tokenize("7px", 0),
		},
		{
			name: "pi",
			args: args{
				s: tokenize("pi()", 0),
			},
			want: tokenize("3.141592653589793", 0),
		},
		{
			name: "no unit may be px",
			args: args{
				s: tokenize("1 + 2px", 0),
			},
			want: tokenize("3px", 0),
		},

		{
			name: "no eval one space",
			args: args{
				s: tokenize("1 +1", 0),
			},
			want: tokenize("1 +1", 0),
		},
		{
			name: "eval one space simple",
			args: args{
				s: tokenize("2- 1", 0),
			},
			want: tokenize("1", 0),
		},
		{
			name: "eval two space simple",
			args: args{
				s: tokenize("2 - 1", 0),
			},
			want: tokenize("1", 0),
		},
		{
			name: "eval one space times",
			args: args{
				s: tokenize("2 *2", 0),
			},
			want: tokenize("4", 0),
		},
		{
			name: "eval two space times",
			args: args{
				s: tokenize("2 * 2", 0),
			},
			want: tokenize("4", 0),
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
