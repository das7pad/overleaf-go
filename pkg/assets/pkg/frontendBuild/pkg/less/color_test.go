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

func Test_evalColor(t *testing.T) {
	type args struct {
		s tokens
	}
	tests := []struct {
		name    string
		args    args
		want    tokens
		wantErr bool
	}{
		{
			name: "hue function",
			args: args{
				s: tokenize("hue(hsl(90, 100%, 50%))", ""),
			},
			want: tokenize("90", ""),
		},
		{
			name: "saturation function",
			args: args{
				s: tokenize("saturation(hsl(90, 100%, 50%))", ""),
			},
			want: tokenize("100%", ""),
		},
		{
			name: "lightness function",
			args: args{
				s: tokenize("lightness(hsl(90, 100%, 50%))", ""),
			},
			want: tokenize("50%", ""),
		},
		{
			name: "red color",
			args: args{
				s: tokenize("dashed red", ""),
			},
			want: tokenize("dashed red", ""),
		},
		{
			name: "red function",
			args: args{
				s: tokenize("red(rgb(10, 20, 30))", ""),
			},
			want: tokenize("10", ""),
		},
		{
			name: "green function",
			args: args{
				s: tokenize("green(rgb(10, 20, 30))", ""),
			},
			want: tokenize("20", ""),
		},
		{
			name: "blue function",
			args: args{
				s: tokenize("blue(rgb(10, 20, 30))", ""),
			},
			want: tokenize("30", ""),
		},
		{
			name: "alpha function",
			args: args{
				s: tokenize("alpha(rgba(10, 20, 30, 0.5))", ""),
			},
			want: tokenize("0.5", ""),
		},
		{
			name: "saturate hsl",
			args: args{
				s: tokenize("saturate(hsl(90, 80%, 50%), 20%)", ""),
			},
			want: tokenize("hsl(90,100%,50%)", ""),
		},
		{
			name: "saturate hex",
			args: args{
				s: tokenize("saturate(#80e619, 20%)", ""),
			},
			want: tokenize("hsl(90,100%,50%)", ""),
		},
		{
			name: "saturate rgb",
			args: args{
				s: tokenize("saturate(rgb(128, 230, 25), 20%)", ""),
			},
			want: tokenize("hsl(90,100%,50%)", ""),
		},
		{
			name: "desaturate",
			args: args{
				s: tokenize("desaturate(hsl(90, 80%, 50%), 20%)", ""),
			},
			want: tokenize("hsl(90,60%,50%)", ""),
		},
		{
			name: "lighten",
			args: args{
				s: tokenize("lighten(hsl(90, 80%, 50%), 20%)", ""),
			},
			want: tokenize("hsl(90,80%,70%)", ""),
		},
		{
			name: "darken",
			args: args{
				s: tokenize("darken(hsl(90, 80%, 50%), 20%)", ""),
			},
			want: tokenize("hsl(90,80%,30%)", ""),
		},
		{
			name: "fade hsla",
			args: args{
				s: tokenize("fade(hsla(90, 90%, 50%, 0.5), 10%)", ""),
			},
			want: tokenize("hsla(90,90%,50%,0.1)", ""),
		},
		{
			name: "fade rgba",
			args: args{
				s: tokenize("fade(rgba(128, 242, 13, 0.5), 10%)", ""),
			},
			want: tokenize("rgba(128,242,13,0.1)", ""),
		},
		{
			name: "fadein",
			args: args{
				s: tokenize("fadein(hsla(90, 90%, 50%, 0.5), 10%)", ""),
			},
			want: tokenize("hsla(90,90%,50%,0.6)", ""),
		},
		{
			name: "fadeout",
			args: args{
				s: tokenize("fadeout(hsla(90, 90%, 50%, 0.5), 10%)", ""),
			},
			want: tokenize("hsla(90,90%,50%,0.4)", ""),
		},
		{
			name: "spin plus",
			args: args{
				s: tokenize("spin(hsl(10, 90%, 50%), 30)", ""),
			},
			want: tokenize("hsl(40,90%,50%)", ""),
		},
		{
			name: "spin minus",
			args: args{
				s: tokenize("spin(hsl(10, 90%, 50%), -30)", ""),
			},
			want: tokenize("hsl(340,90%,50%)", ""),
		},
		{
			name: "mix",
			args: args{
				s: tokenize("mix(#ff0000, #0000ff, 50%)", ""),
			},
			want: tokenize("#800080", ""),
		},
		{
			name: "tint",
			args: args{
				s: tokenize("tint(#007fff, 50%)", ""),
			},
			want: tokenize("#80bfff", ""),
		},
		{
			name: "tint alpha",
			args: args{
				s: tokenize("tint(rgba(0, 0, 255, 0.5), 50%)", ""),
			},
			want: tokenize("rgba(191,191,255,0.75)", ""),
		},
		{
			name: "shade",
			args: args{
				s: tokenize("shade(#007fff, 50%)", ""),
			},
			want: tokenize("#004080", ""),
		},
		{
			name: "shade alpha",
			args: args{
				s: tokenize("shade(rgba(0, 0, 255, 0.5), 50%)", ""),
			},
			want: tokenize("rgba(0,0,64,0.75)", ""),
		},
		{
			name: "greyscale",
			args: args{
				s: tokenize("greyscale(hsl(90, 90%, 50%))", ""),
			},
			want: tokenize("hsl(90,0%,50%)", ""),
		},
		{
			name: "compose color",
			args: args{
				s: tokenize("rgba(red(#123), green(#123), blue(#123), 0.5)", ""),
			},
			want: tokenize("rgba(17, 34, 51, 0.5)", ""),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := evalColor(tt.args.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("evalColor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.String() != tt.want.String() {
				t.Errorf("evalColor() got = %v, want %v", got, tt.want)
			}
		})
	}
}
