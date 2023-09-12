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
	"reflect"
	"testing"
)

func Test_tokenize(t *testing.T) {
	type args struct {
		s string
		f int16
	}
	tests := []struct {
		name string
		args args
		want tokens
	}{
		{
			name: "one line",
			args: args{
				s: ".x{color:red;}",
				f: 0,
			},
			want: []token{
				{kind: tokenDot, line: 0, column: 0, v: "."},
				{kind: tokenIdentifier, line: 0, column: 1, v: "x"},
				{kind: tokenCurlyOpen, line: 0, column: 2, v: "{"},
				{kind: tokenIdentifier, line: 0, column: 3, v: "color"},
				{kind: tokenColon, line: 0, column: 8, v: ":"},
				{kind: tokenIdentifier, line: 0, column: 9, v: "red"},
				{kind: tokenSemi, line: 0, column: 12, v: ";"},
				{kind: tokenCurlyClose, line: 0, column: 13, v: "}"},
			},
		},
		{
			name: "two lines",
			args: args{
				s: ".x{\n  color: red;\n}",
				f: 0,
			},
			want: []token{
				{kind: tokenDot, line: 0, column: 0, v: "."},
				{kind: tokenIdentifier, line: 0, column: 1, v: "x"},
				{kind: tokenCurlyOpen, line: 0, column: 2, v: "{"},
				{kind: tokenNewline, line: 0, column: 3, v: "\n"},
				{kind: space, line: 1, column: 0, v: "  "},
				{kind: tokenIdentifier, line: 1, column: 2, v: "color"},
				{kind: tokenColon, line: 1, column: 7, v: ":"},
				{kind: space, line: 1, column: 8, v: " "},
				{kind: tokenIdentifier, line: 1, column: 9, v: "red"},
				{kind: tokenSemi, line: 1, column: 12, v: ";"},
				{kind: tokenNewline, line: 1, column: 13, v: "\n"},
				{kind: tokenCurlyClose, line: 2, column: 0, v: "}"},
			},
		},
		{
			name: "single line comment",
			args: args{
				s: ".x{\n  color: red; // foo\n}",
				f: 0,
			},
			want: []token{
				{kind: tokenDot, line: 0, column: 0, v: "."},
				{kind: tokenIdentifier, line: 0, column: 1, v: "x"},
				{kind: tokenCurlyOpen, line: 0, column: 2, v: "{"},
				{kind: tokenNewline, line: 0, column: 3, v: "\n"},
				{kind: space, line: 1, column: 0, v: "  "},
				{kind: tokenIdentifier, line: 1, column: 2, v: "color"},
				{kind: tokenColon, line: 1, column: 7, v: ":"},
				{kind: space, line: 1, column: 8, v: " "},
				{kind: tokenIdentifier, line: 1, column: 9, v: "red"},
				{kind: tokenSemi, line: 1, column: 12, v: ";"},
				{kind: space, line: 1, column: 13, v: " "},
				{kind: tokenCurlyClose, line: 2, column: 0, v: "}"},
			},
		},
		{
			name: "single line comment env",
			args: args{
				s: ".x{\n  color: red; /* foo */}",
				f: 0,
			},
			want: []token{
				{kind: tokenDot, line: 0, column: 0, v: "."},
				{kind: tokenIdentifier, line: 0, column: 1, v: "x"},
				{kind: tokenCurlyOpen, line: 0, column: 2, v: "{"},
				{kind: tokenNewline, line: 0, column: 3, v: "\n"},
				{kind: space, line: 1, column: 0, v: "  "},
				{kind: tokenIdentifier, line: 1, column: 2, v: "color"},
				{kind: tokenColon, line: 1, column: 7, v: ":"},
				{kind: space, line: 1, column: 8, v: " "},
				{kind: tokenIdentifier, line: 1, column: 9, v: "red"},
				{kind: tokenSemi, line: 1, column: 12, v: ";"},
				{kind: space, line: 1, column: 13, v: " "},
				{kind: tokenCurlyClose, line: 1, column: 23, v: "}"},
			},
		},
		{
			name: "multi line comment env",
			args: args{
				s: ".x{\n  color: red; /* foo\n*/\n}",
				f: 0,
			},
			want: []token{
				{kind: tokenDot, line: 0, column: 0, v: "."},
				{kind: tokenIdentifier, line: 0, column: 1, v: "x"},
				{kind: tokenCurlyOpen, line: 0, column: 2, v: "{"},
				{kind: tokenNewline, line: 0, column: 3, v: "\n"},
				{kind: space, line: 1, column: 0, v: "  "},
				{kind: tokenIdentifier, line: 1, column: 2, v: "color"},
				{kind: tokenColon, line: 1, column: 7, v: ":"},
				{kind: space, line: 1, column: 8, v: " "},
				{kind: tokenIdentifier, line: 1, column: 9, v: "red"},
				{kind: tokenSemi, line: 1, column: 12, v: ";"},
				{kind: space, line: 1, column: 13, v: " "},
				{kind: tokenNewline, line: 2, column: 2, v: "\n"},
				{kind: tokenCurlyClose, line: 3, column: 0, v: "}"},
			},
		},
		{
			name: "multi line comment env trailing",
			args: args{
				s: ".x{\n  color: red; /* foo\n   */\n}",
				f: 0,
			},
			want: []token{
				{kind: tokenDot, line: 0, column: 0, v: "."},
				{kind: tokenIdentifier, line: 0, column: 1, v: "x"},
				{kind: tokenCurlyOpen, line: 0, column: 2, v: "{"},
				{kind: tokenNewline, line: 0, column: 3, v: "\n"},
				{kind: space, line: 1, column: 0, v: "  "},
				{kind: tokenIdentifier, line: 1, column: 2, v: "color"},
				{kind: tokenColon, line: 1, column: 7, v: ":"},
				{kind: space, line: 1, column: 8, v: " "},
				{kind: tokenIdentifier, line: 1, column: 9, v: "red"},
				{kind: tokenSemi, line: 1, column: 12, v: ";"},
				{kind: space, line: 1, column: 13, v: " "},
				{kind: tokenNewline, line: 2, column: 5, v: "\n"},
				{kind: tokenCurlyClose, line: 3, column: 0, v: "}"},
			},
		},
		{
			name: "many line comment env",
			args: args{
				s: ".x{\n  color: red; /*\nfoo\nbar\nbaz\n*/\n}",
				f: 0,
			},
			want: []token{
				{kind: tokenDot, line: 0, column: 0, v: "."},
				{kind: tokenIdentifier, line: 0, column: 1, v: "x"},
				{kind: tokenCurlyOpen, line: 0, column: 2, v: "{"},
				{kind: tokenNewline, line: 0, column: 3, v: "\n"},
				{kind: space, line: 1, column: 0, v: "  "},
				{kind: tokenIdentifier, line: 1, column: 2, v: "color"},
				{kind: tokenColon, line: 1, column: 7, v: ":"},
				{kind: space, line: 1, column: 8, v: " "},
				{kind: tokenIdentifier, line: 1, column: 9, v: "red"},
				{kind: tokenSemi, line: 1, column: 12, v: ";"},
				{kind: space, line: 1, column: 13, v: " "},
				{kind: tokenNewline, line: 5, column: 2, v: "\n"},
				{kind: tokenCurlyClose, line: 6, column: 0, v: "}"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tokenize(tt.args.s, tt.args.f); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("tokenize() = \n%v, want \n%v", got, tt.want)
			}
		})
	}
}
