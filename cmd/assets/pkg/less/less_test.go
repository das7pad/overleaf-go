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
	"io/fs"
	"reflect"
	"testing"
)

type fakeFS map[string]string

func (f fakeFS) ReadFile(name string) ([]byte, error) {
	s, ok := f[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return []byte(s), nil
}

func TestParseUsing(t *testing.T) {
	type args struct {
		read func(name string) ([]byte, error)
		p    string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		want1   []string
		wantErr bool
	}{
		{
			name: "plain css single directive",
			args: args{
				read: fakeFS{
					"in.less": ".btn { color: red; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; }",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "plain css multiple directives",
			args: args{
				read: fakeFS{
					"in.less": "html { color: red; top: 1px; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    "html { color: red; top: 1px; }",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "plain css nesting",
			args: args{
				read: fakeFS{
					"in.less": "html { color: blue; .btn { color: red; } }",
				}.ReadFile,
				p: "in.less",
			},
			want:    "html { color: blue; .btn { color: red; } }",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "single line comment",
			args: args{
				read: fakeFS{
					"in.less": ".btn { // color: red;\n color: blue; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: blue; }",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "multi line comment",
			args: args{
				read: fakeFS{
					"in.less": ".btn { /*\n color: red;\n */ color: blue; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: blue; }",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "variable same file",
			args: args{
				read: fakeFS{
					"in.less": "@red: red; .btn { color: @red; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; }",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "variable other file",
			args: args{
				read: fakeFS{
					"in.less":    "@import 'other.less'; .btn { color: @red; }",
					"other.less": "@red: red;",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; }",
			want1:   []string{"in.less", "other.less"},
			wantErr: false,
		},
		{
			name: "variable overwrite",
			args: args{
				read: fakeFS{
					"in.less":    "@red: blue; @import 'other.less'; .btn { color: @red; }",
					"other.less": "@red: red;",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; }",
			want1:   []string{"in.less", "other.less"},
			wantErr: false,
		},
		{
			name: "variable chain",
			args: args{
				read: fakeFS{
					"in.less": "@red: @r1; @r1: red; .btn { color: @red; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; }",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "variable nested",
			args: args{
				read: fakeFS{
					"in.less": ".red { @c: red; color: @c; } .blue { @c: blue; color: @c; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".red { color: red; } .blue { color: blue; }",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "when true boolean",
			args: args{
				read: fakeFS{
					"in.less": "@c: true; .btn when (@c = true) { color: red; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; }",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "when false boolean",
			args: args{
				read: fakeFS{
					"in.less": "@c: true; .btn when (@c = false) { color: red; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    "",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "when true int",
			args: args{
				read: fakeFS{
					"in.less": "@c: 1; .btn when (@c <= 2) { color: red; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; }",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "when false int",
			args: args{
				read: fakeFS{
					"in.less": "@c: 100; .btn when (@c <= 2) { color: red; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    "",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "when true multiple",
			args: args{
				read: fakeFS{
					"in.less": "@c1: 1; @c2: true; .btn when (@c1 <= 2) and (@c2 = true) { color: red; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; }",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "when false multiple",
			args: args{
				read: fakeFS{
					"in.less": "@c1: 1; @c2: false; .btn when (@c1 <= 2) and (@c2 = true) { color: red; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    "",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "mixin no args",
			args: args{
				read: fakeFS{
					"in.less": ".red() { color: red; } .btn { .red(); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; }",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "mixin static arg",
			args: args{
				read: fakeFS{
					"in.less": ".red(@c) { color: @c; } .btn { .red(red); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; }",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "mixin static arg use global",
			args: args{
				read: fakeFS{
					"in.less": "@blue: blue; .red(@c) { color: @c; background-color: @blue; } .btn { .red(red); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; background-color: blue; }",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "mixin var arg",
			args: args{
				read: fakeFS{
					"in.less": "@red: red; .red(@c) { color: @c; } .btn { .red(@red); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; }",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "mixin match arg",
			args: args{
				read: fakeFS{
					"in.less": ".red(red) { color: red; } .red(blue) { color: blue; } .btn { .red(red); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; }",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "mixin two args comma",
			args: args{
				read: fakeFS{
					"in.less": ".red(@c1, @c2) { color: @c1; padding: @c2; } .btn { .red(red, 2px); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; padding: 2px; }",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "mixin two args semicolon",
			args: args{
				read: fakeFS{
					"in.less": ".red(@c1; @c2) { color: @c1; padding: @c2; } .btn { .red(red; 2px); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; padding: 2px; }",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "mixin function call arg",
			args: args{
				read: fakeFS{
					"in.less": ".red(@c1, @c2) { color: @c1; padding: @c2; } .btn { .red(rgb(255, 0, 0), 2px); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: rgb(255, 0, 0); padding: 2px; }",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "mixin named",
			args: args{
				read: fakeFS{
					"in.less": ".red(@c1, @c2) { color: @c1; padding: @c2; } .btn { .red(@c2: 2px, @c1: red); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; padding: 2px; }",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "mixin default",
			args: args{
				read: fakeFS{
					"in.less": ".red(@c1, @c2: 2px) { color: @c1; padding: @c2; } .btn { .red(red); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; padding: 2px; }",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "mixin two args match one",
			args: args{
				read: fakeFS{
					"in.less": ".red(@c1, @c2) { color: @c1; padding: @c2; } .red(blue, @c2) { background-color: blue; margin: @c2; } .btn { .red(red, 2px); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; padding: 2px; }",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "mixin two args match both",
			args: args{
				read: fakeFS{
					"in.less": ".red(@c1, @c2) { color: @c1; padding: @c2; } .red(red, @c2) { background-color: red; margin: @c2; } .btn { .red(red, 2px); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; padding: 2px; background-color: red; margin: 2px; }",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "mixin named class",
			args: args{
				read: fakeFS{
					"in.less": ".gen(@var) { &.@{var} { color: @var; } } .render { .gen(red); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".render { &.red { color: red; } }",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "mixin two named classes",
			args: args{
				read: fakeFS{
					"in.less": ".gen(@var) { &.@{var} { color: @var; } } .render { .gen(red); .gen(blue); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".render { &.red { color: red; } &.blue { color: blue; } }",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "mixin when",
			args: args{
				read: fakeFS{
					"in.less": ".x(@c) when (@c = 1) { padding: @c; } .x(@c) when (@c = 2) { margin: @c; } .one { .x(1); } .two { .x(2); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".one { padding: 1; } .two { margin: 2; }",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "mixin nested name",
			args: args{
				read: fakeFS{
					"in.less": "#foo { .red() { color: red; } } .btn { #foo > .red(); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; }",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "mixin nested empty pint",
			args: args{
				read: fakeFS{
					"in.less": "#foo { .red() { color: red; } }",
				}.ReadFile,
				p: "in.less",
			},
			want:    "",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "keyframes",
			args: args{
				read: fakeFS{
					"in.less": "@keyframes foo { 0% {} 50% { background-color: transparent; } 100% {} }",
				}.ReadFile,
				p: "in.less",
			},
			want:    "@keyframes foo { 0% { } 50% { background-color: transparent; } 100% { } }",
			want1:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "import less",
			args: args{
				read: fakeFS{
					"in.less":    "@import 'other.less';",
					"other.less": ".btn { color: red; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; }",
			want1:   []string{"in.less", "other.less"},
			wantErr: false,
		},
		{
			name: "import css",
			args: args{
				read: fakeFS{
					"in.less":   "@import (less) 'other.css';",
					"other.css": ".btn { color: red; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; }",
			want1:   []string{"in.less", "other.css"},
			wantErr: false,
		},
		{
			name: "import parent folder",
			args: args{
				read: fakeFS{
					"foo/bar/baz.less":  "@import '../../public/other.less';",
					"public/other.less": ".btn { color: red; }",
				}.ReadFile,
				p: "foo/bar/baz.less",
			},
			want:    ".btn { color: red; }",
			want1:   []string{"foo/bar/baz.less", "public/other.less"},
			wantErr: false,
		},
		{
			name: "import order",
			args: args{
				read: fakeFS{
					"in.less":  "@import 'one.less'; .btn { color: green; } @import 'two.less';",
					"one.less": ".btn { color: red; }",
					"two.less": ".btn { color: blue; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; } .btn { color: green; } .btn { color: blue; }",
			want1:   []string{"in.less", "one.less", "two.less"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := ParseUsing(tt.args.read, tt.args.p)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseUsing() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseUsing() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("ParseUsing() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
