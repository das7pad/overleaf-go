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
