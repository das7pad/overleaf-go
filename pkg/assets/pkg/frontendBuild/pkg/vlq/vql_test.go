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

package vlq

import (
	"testing"
)

func TestEncode(t *testing.T) {
	type args struct {
		value int32
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "zero",
			args: args{
				value: 0,
			},
			want: "A",
		},
		{
			name: "one",
			args: args{
				value: 1,
			},
			want: "C",
		},
		{
			name: "two",
			args: args{
				value: 2,
			},
			want: "E",
		},
		{
			name: "three",
			args: args{
				value: 3,
			},
			want: "G",
		},
		{
			name: "fifteen",
			args: args{
				value: 15,
			},
			want: "e",
		},
		{
			name: "sixteen",
			args: args{
				value: 16,
			},
			want: "gB",
		},
		{
			name: "seventeen",
			args: args{
				value: 17,
			},
			want: "iB",
		},
		{
			name: "thirtyOne",
			args: args{
				value: 31,
			},
			want: "+B",
		},
		{
			name: "thirtyTwo",
			args: args{
				value: 32,
			},
			want: "gC",
		},
		{
			name: "thirtyThree",
			args: args{
				value: 33,
			},
			want: "iC",
		},
		{
			name: "fortyTwo",
			args: args{
				value: 42,
			},
			want: "0C",
		},
		{
			name: "fortySeven",
			args: args{
				value: 47,
			},
			want: "+C",
		},
		{
			name: "fortyEight",
			args: args{
				value: 48,
			},
			want: "gD",
		},
		{
			name: "fortyNine",
			args: args{
				value: 49,
			},
			want: "iD",
		},
		{
			name: "sixtyThree",
			args: args{
				value: 63,
			},
			want: "+D",
		},
		{
			name: "sixtyFour",
			args: args{
				value: 64,
			},
			want: "gE",
		},
		{
			name: "sixtyFive",
			args: args{
				value: 65,
			},
			want: "iE",
		},
		{
			name: "1337",
			args: args{
				value: 1337,
			},
			want: "yzC",
		},
		{
			name: "minusOne",
			args: args{
				value: -1,
			},
			want: "D",
		},
		{
			name: "minusTwo",
			args: args{
				value: -2,
			},
			want: "F",
		},
		{
			name: "minusSixteen",
			args: args{
				value: -16,
			},
			want: "hB",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := Encode(nil, tt.args.value)
			if got := string(b); got != tt.want {
				t.Errorf("Encode() = %v, want %v", got, tt.want)
			}
		})
	}
}
