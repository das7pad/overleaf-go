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

package translations

import (
	"html/template"
	"testing"
)

func TestTranslateInto(t *testing.T) {
	type args struct {
		language string
		key      string
		pairs    []string
	}
	tests := []struct {
		name string
		args args
		want template.HTML
	}{
		{
			name: "trivial",
			args: args{
				language: "en",
				key:      "log_in",
			},
			want: "Log In",
		},
		{
			name: "substitute var",
			args: args{
				language: "en",
				key:      "go_page",
				pairs:    []string{"page", "42"},
			},
			want: "Go to page 42",
		},
		{
			name: "substitute var and HTML",
			args: args{
				language: "en",
				key:      "click_here_to_view_sl_in_lng",
				pairs: []string{
					"appName", "Overleaf",
					"lngName", "English",
					"0", "strong",
				},
			},
			want: "Click here to use Overleaf in <strong>English</strong>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := TranslateInto(tt.args.language)
			if got := fn(tt.args.key, tt.args.pairs...); got != tt.want {
				t.Errorf("TranslateInto() = %v, want %v", got, tt.want)
			}
		})
	}
}
