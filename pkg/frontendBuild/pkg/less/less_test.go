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
		want1   string
		want2   []string
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
			want2:   []string{"in.less"},
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
			want2:   []string{"in.less"},
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
			want:    "html { color: blue; } html .btn { color: red; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "plain css nesting amp",
			args: args{
				read: fakeFS{
					"in.less": ".foo { color: blue; body > button& { color: red; } }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".foo { color: blue; } body > button.foo { color: red; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "plain css nesting multiple",
			args: args{
				read: fakeFS{
					"in.less": ".foo, .bar { color: blue; .baz { color: red; } }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".foo,.bar { color: blue; } .foo .baz,.bar .baz { color: red; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "plain css nesting amp mixed",
			args: args{
				read: fakeFS{
					"in.less": ".foo { color: blue; body > button&,.btn &,.bar, &:after { color: red; } }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".foo { color: blue; } body > button.foo,.btn .foo,.foo .bar,.foo:after { color: red; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "matcher multi-line",
			args: args{
				read: fakeFS{
					"in.less": "html\n .btn { color: red; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    "html .btn { color: red; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "directive value multi-line",
			args: args{
				read: fakeFS{
					"in.less": ".btn { margin: 1px\n 2px; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { margin: 1px 2px; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "directive name multi-line",
			args: args{
				read: fakeFS{
					"in.less": "#x { .y() { color: red; } } .btn { #x >\n .y(); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "extend nested",
			args: args{
				read: fakeFS{
					"in.less": ".red { color: red; } .btn { &:extend(.red); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".red,.btn { color: red; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "extend nested deep",
			args: args{
				read: fakeFS{
					"in.less": ".red { color: red; } .btn { .x { .y { &:extend(.red); } } }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".red,.btn .x .y { color: red; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "extend mixin",
			args: args{
				read: fakeFS{
					"in.less": ".red { color: red; } .btn { .x { .y { .z(); } } } .z() { &:extend(.red); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".red,.btn .x .y { color: red; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "extend matcher",
			args: args{
				read: fakeFS{
					"in.less": ".red { color: red; } .btn:extend(.red) { margin: 1px; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".red,.btn { color: red; } .btn { margin: 1px; }",
			want2:   []string{"in.less"},
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
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "end comment //",
			args: args{
				read: fakeFS{
					"in.less": ".btn { color: red; } // end",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "end comment /*",
			args: args{
				read: fakeFS{
					"in.less": ".btn { color: red; } /* end */",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "escaped comment",
			args: args{
				read: fakeFS{
					"in.less": ".btn { content: '\\// foo'; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { content: '\\// foo'; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "header comment //",
			args: args{
				read: fakeFS{
					"in.less": "// header\n.btn { color: red; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "header comment /*",
			args: args{
				read: fakeFS{
					"in.less": "/* header\n */.btn { color: red; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "multi line comment",
			args: args{
				read: fakeFS{
					"in.less": ".btn {/*\n color: red;\n */color: blue; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: blue; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "filter raw",
			args: args{
				read: fakeFS{
					"other.css": ".btn { filter: alpha(opacity=50); }",
					"in.less":   "@import (less) 'other.css';",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { filter: alpha(opacity=50); }",
			want2:   []string{"in.less", "other.css"},
			wantErr: false,
		},
		{
			name: "filter string unwrap",
			args: args{
				read: fakeFS{
					"in.less": "@x: 50; .btn { filter: ~'alpha(opacity=@{x})'; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { filter: alpha(opacity=50); }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "calc plain",
			args: args{
				read: fakeFS{
					"in.less": ".btn { margin: calc(100vh - 1px); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { margin: calc(100vh - 1px); }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "calc string unwrap single",
			args: args{
				read: fakeFS{
					"in.less": ".btn { margin: calc(~'100vh - 1px'); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { margin: calc(100vh - 1px); }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "calc string unwrap single math",
			args: args{
				read: fakeFS{
					"in.less": ".btn { margin: calc(~'100vh - ' (1px + 1px)); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { margin: calc(100vh - 2px); }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "calc string unwrap multiple",
			args: args{
				read: fakeFS{
					"in.less": "@x: 1px; .btn { margin: calc(~'100vh - ' @x ~' - 1rem'); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { margin: calc(100vh - 1px  - 1rem); }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "calc string unwrap important",
			args: args{
				read: fakeFS{
					"in.less": ".btn { margin: calc(~'100vh - 1px') !important; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { margin: calc(100vh - 1px) !important; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "string unwrap keep space",
			args: args{
				read: fakeFS{
					"in.less": ".btn { padding: ~'1' (1+1) ~'3'; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { padding: 1 2 3; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "math single operation",
			args: args{
				read: fakeFS{
					"in.less": ".btn { margin: 1+1; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { margin: 2; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "math single operation after numbers",
			args: args{
				read: fakeFS{
					"in.less": ".btn { padding: 1 (1+1) 3; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { padding: 1 2 3; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "math single operation after numbers new line",
			args: args{
				read: fakeFS{
					"in.less": "@x: (2-1) 2\n 3; .btn { padding: @x; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { padding: 1 2\n 3; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "math single operation important",
			args: args{
				read: fakeFS{
					"in.less": ".btn { margin: 1+1 !important; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { margin: 2 !important; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "math single operation same unit",
			args: args{
				read: fakeFS{
					"in.less": ".btn { margin: 1px + 1px; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { margin: 2px; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "math times unit first",
			args: args{
				read: fakeFS{
					"in.less": ".btn { margin: 2px * 2; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { margin: 4px; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "math times unit second",
			args: args{
				read: fakeFS{
					"in.less": ".btn { margin: 2 * 2px; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { margin: 4px; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "math div unit first",
			args: args{
				read: fakeFS{
					"in.less": ".btn { margin: 2px / 2; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { margin: 1px; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "math div unit both",
			args: args{
				read: fakeFS{
					"in.less": ".btn { margin: 6px / 3px; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { margin: 2; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "math single operation different unit pass through",
			args: args{
				read: fakeFS{
					"in.less": ".btn { margin: 1rem + 1px; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { margin: 1rem + 1px; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "math single operation parens",
			args: args{
				read: fakeFS{
					"in.less": ".btn { margin: (1px + 1px); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { margin: 2px; }",
			want2:   []string{"in.less"},
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
			want2:   []string{"in.less"},
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
			want2:   []string{"in.less", "other.less"},
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
			want2:   []string{"in.less", "other.less"},
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
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "variable variable",
			args: args{
				read: fakeFS{
					"in.less": "@l1: l2; @l2: red; .btn { color: @@l1; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "variable variable variable variable",
			args: args{
				read: fakeFS{
					"in.less": "@l1: l2; @l2: l3; @l3: l4; @l4: red; .btn { color: @@@@l1; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; }",
			want2:   []string{"in.less"},
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
			want2:   []string{"in.less"},
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
			want2:   []string{"in.less"},
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
			want2:   []string{"in.less"},
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
			want2:   []string{"in.less"},
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
			want2:   []string{"in.less"},
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
			want2:   []string{"in.less"},
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
			want2:   []string{"in.less"},
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
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "mixin @var",
			args: args{
				read: fakeFS{
					"in.less": "@red: { color: red; }; .btn { @red(); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "mixin no parens",
			args: args{
				read: fakeFS{
					"in.less": ".red() { color: red; } .btn { .red; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "mixin string unwrap",
			args: args{
				read: fakeFS{
					"in.less": ".gen(@m) { @{m} { color: red; } } .gen(~'.red, .btn-error');",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".red,.btn-error { color: red; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "mixin other class",
			args: args{
				read: fakeFS{
					"in.less": ".red { color: red; } .btn { .red(); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".red { color: red; } .btn { color: red; }",
			want2:   []string{"in.less"},
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
			want2:   []string{"in.less"},
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
			want2:   []string{"in.less"},
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
			want2:   []string{"in.less"},
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
			want2:   []string{"in.less"},
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
			want2:   []string{"in.less"},
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
			want2:   []string{"in.less"},
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
			want2:   []string{"in.less"},
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
			want2:   []string{"in.less"},
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
			want2:   []string{"in.less"},
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
			want2:   []string{"in.less"},
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
			want2:   []string{"in.less"},
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
			want:    ".render.red { color: red; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "mixin named class many vars",
			args: args{
				read: fakeFS{
					"in.less": ".gen(@c1, @c2, @c3, @c4) { &.p-@{c1}-@{c2}-@{c3}-@{c4} { padding: @c1 @c2 @c3 @c4; } } .render { .gen(1, 2, 3, 4); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".render.p-1-2-3-4 { padding: 1 2 3 4; }",
			want2:   []string{"in.less"},
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
			want:    ".render.red { color: red; } .render.blue { color: blue; }",
			want2:   []string{"in.less"},
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
			want2:   []string{"in.less"},
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
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "mixin nested empty print",
			args: args{
				read: fakeFS{
					"in.less": "#foo { .red() { color: red; } }",
				}.ReadFile,
				p: "in.less",
			},
			want:    "",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "mixin nested same name",
			args: args{
				read: fakeFS{
					"in.less": ".red { .cell() { color: red; } .cell(); } .blue { .cell() { color: blue; } .cell(); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".red { color: red; } .blue { color: blue; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "mixin recursion",
			args: args{
				read: fakeFS{
					"in.less": ".m(@c) when (@c = 1) { color: red; } .m(@c) when (@c > 1) { .m(@c - 1); } .red { .m(2); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".red { color: red; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "mixin local var",
			args: args{
				read: fakeFS{
					"in.less": "@x: 1; @y: 2; .mix(@a) { @sum: @a+1; padding: @sum; } .btn { .mix(@x); .foo { .mix(@y); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { padding: 2; } .btn .foo { padding: 3; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "mixin invocation order",
			args: args{
				read: fakeFS{
					"in.less": ".gen-red() { .red { color: red; } } .btn { padding: 3; } .gen-red();",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { padding: 3; } .red { color: red; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "each",
			args: args{
				read: fakeFS{
					"in.less": "@foo: { 1: 1px; 2: 2px; }; each(@foo, { .col-@{key} { padding: @value; } });",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".col-1 { padding: 1px; } .col-2 { padding: 2px; }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "keyframes",
			args: args{
				read: fakeFS{
					"in.less": "@keyframes foo { 0% { background-color: red; } 50% { background-color: transparent; } 100% {} }",
				}.ReadFile,
				p: "in.less",
			},
			want:    "@keyframes foo { 0% { background-color: red; } 50% { background-color: transparent; } 100% { } }",
			want2:   []string{"in.less"},
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
			want2:   []string{"in.less", "other.less"},
			wantErr: false,
		},
		{
			name: "import no ext",
			args: args{
				read: fakeFS{
					"in.less":    "@import 'other';",
					"other.less": ".btn { color: red; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; }",
			want2:   []string{"in.less", "other.less"},
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
			want2:   []string{"in.less", "other.css"},
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
			want2:   []string{"foo/bar/baz.less", "public/other.less"},
			wantErr: false,
		},
		{
			name: "import nested folder",
			args: args{
				read: fakeFS{
					"in.less":            "@import 'foo/bar/baz.less';",
					"foo/bar/baz.less":   "@import 'other.less';",
					"foo/bar/other.less": ".btn { color: red; }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { color: red; }",
			want2:   []string{"in.less", "foo/bar/baz.less", "foo/bar/other.less"},
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
			want2:   []string{"in.less", "one.less", "two.less"},
			wantErr: false,
		},
		{
			name: "url nested folder no quotes",
			args: args{
				read: fakeFS{
					"in.less":          "@import 'foo/bar/baz.less';",
					"foo/bar/baz.less": ".btn { background-image: url(img.png); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { background-image: url(foo/bar/img.png); }",
			want2:   []string{"in.less", "foo/bar/baz.less"},
			wantErr: false,
		},
		{
			name: "url nested folder with single quotes",
			args: args{
				read: fakeFS{
					"in.less":          "@import 'foo/bar/baz.less';",
					"foo/bar/baz.less": ".btn { background-image: url('img.png'); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { background-image: url('foo/bar/img.png'); }",
			want2:   []string{"in.less", "foo/bar/baz.less"},
			wantErr: false,
		},
		{
			name: "url nested folder with double quotes",
			args: args{
				read: fakeFS{
					"in.less":          "@import 'foo/bar/baz.less';",
					"foo/bar/baz.less": ".btn { background-image: url(\"img.png\"); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { background-image: url(\"foo/bar/img.png\"); }",
			want2:   []string{"in.less", "foo/bar/baz.less"},
			wantErr: false,
		},
		{
			name: "url parent folder",
			args: args{
				read: fakeFS{
					"in.less": ".btn { background-image: url(../../public/img.png); }",
				}.ReadFile,
				p: "in.less",
			},
			want:    ".btn { background-image: url(../../public/img.png); }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "charset",
			args: args{
				read: fakeFS{
					"in.less":  "@import 'one.less'; @import 'two.less';",
					"one.less": ".btn { color: red; }",
					"two.less": "@charset \"UTF-8\";",
				}.ReadFile,
				p: "in.less",
			},
			want:    "@charset \"UTF-8\"; .btn { color: red; }",
			want2:   []string{"in.less", "one.less", "two.less"},
			wantErr: false,
		},
		{
			name: "media print",
			args: args{
				read: fakeFS{
					"in.less": "@media print { .btn { color: red; } }",
				}.ReadFile,
				p: "in.less",
			},
			want:    "@media print { .btn { color: red; } }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "media only screen",
			args: args{
				read: fakeFS{
					"in.less": "@media only screen { .btn { color: red; } }",
				}.ReadFile,
				p: "in.less",
			},
			want:    "@media only screen { .btn { color: red; } }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "media one condition",
			args: args{
				read: fakeFS{
					"in.less": "@media (max-width: 1px) { .btn { color: red; } }",
				}.ReadFile,
				p: "in.less",
			},
			want:    "@media (max-width: 1px) { .btn { color: red; } }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "media two conditions",
			args: args{
				read: fakeFS{
					"in.less": "@media (min-width: 1px) and (max-width: 2px) { .btn { color: red; } }",
				}.ReadFile,
				p: "in.less",
			},
			want:    "@media (min-width: 1px) and (max-width: 2px) { .btn { color: red; } }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
		{
			name: "media multiple matches",
			args: args{
				read: fakeFS{
					"in.less": "@media only screen and (min-width: 1px), only screen and (min-resolution: 1dpi) { .btn { color: red; } }",
				}.ReadFile,
				p: "in.less",
			},
			want:    "@media only screen and (min-width: 1px), only screen and (min-resolution: 1dpi) { .btn { color: red; } }",
			want2:   []string{"in.less"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, got2, err := ParseUsing(tt.args.read, tt.args.p)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseUsing() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseUsing() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got2, tt.want2) {
				t.Errorf("ParseUsing() got2 = %v, want %v", got2, tt.want2)
			}
		})
	}
}
