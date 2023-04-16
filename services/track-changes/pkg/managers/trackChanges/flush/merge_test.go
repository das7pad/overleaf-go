// Golang port of Overleaf
// Copyright (C) 2022-2023 Jakob Ackermann <das7pad@outlook.com>
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

package flush

import (
	"reflect"
	"testing"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func Test_mergeComponents(t *testing.T) {
	type args struct {
		a sharedTypes.Component
		b sharedTypes.Component
	}
	tests := []struct {
		name  string
		args  args
		want  bool
		wantC sharedTypes.Component
	}{
		{
			name: "i-i append",
			args: args{
				a: sharedTypes.Component{
					Position:  1,
					Insertion: sharedTypes.Snippet("a"),
				},
				b: sharedTypes.Component{
					Position:  2,
					Insertion: sharedTypes.Snippet("b"),
				},
			},
			want: true,
			wantC: sharedTypes.Component{
				Position:  1,
				Insertion: sharedTypes.Snippet("ab"),
			},
		},
		{
			name: "i-i prepend",
			args: args{
				a: sharedTypes.Component{
					Position:  1,
					Insertion: sharedTypes.Snippet("a"),
				},
				b: sharedTypes.Component{
					Position:  1,
					Insertion: sharedTypes.Snippet("b"),
				},
			},
			want: true,
			wantC: sharedTypes.Component{
				Position:  1,
				Insertion: sharedTypes.Snippet("ba"),
			},
		},
		{
			name: "i-i insert a",
			args: args{
				a: sharedTypes.Component{
					Position:  1,
					Insertion: sharedTypes.Snippet("ac"),
				},
				b: sharedTypes.Component{
					Position:  2,
					Insertion: sharedTypes.Snippet("b"),
				},
			},
			want: true,
			wantC: sharedTypes.Component{
				Position:  1,
				Insertion: sharedTypes.Snippet("abc"),
			},
		},
		{
			name: "i-i split",
			args: args{
				a: sharedTypes.Component{
					Position:  1,
					Insertion: sharedTypes.Snippet("a"),
				},
				b: sharedTypes.Component{
					Position:  3,
					Insertion: sharedTypes.Snippet("b"),
				},
			},
			want: false,
			wantC: sharedTypes.Component{
				Position:  1,
				Insertion: sharedTypes.Snippet("a"),
			},
		},
		{
			name: "i-d revert",
			args: args{
				a: sharedTypes.Component{
					Position:  1,
					Insertion: sharedTypes.Snippet("a"),
				},
				b: sharedTypes.Component{
					Position: 1,
					Deletion: sharedTypes.Snippet("a"),
				},
			},
			want: true,
			wantC: sharedTypes.Component{
				Position:  1,
				Insertion: sharedTypes.Snippet(""),
			},
		},
		{
			name: "i-d a-larger start",
			args: args{
				a: sharedTypes.Component{
					Position:  1,
					Insertion: sharedTypes.Snippet("abc"),
				},
				b: sharedTypes.Component{
					Position: 1,
					Deletion: sharedTypes.Snippet("a"),
				},
			},
			want: true,
			wantC: sharedTypes.Component{
				Position:  1,
				Insertion: sharedTypes.Snippet("bc"),
			},
		},
		{
			name: "i-d a-larger center",
			args: args{
				a: sharedTypes.Component{
					Position:  1,
					Insertion: sharedTypes.Snippet("abc"),
				},
				b: sharedTypes.Component{
					Position: 2,
					Deletion: sharedTypes.Snippet("b"),
				},
			},
			want: true,
			wantC: sharedTypes.Component{
				Position:  1,
				Insertion: sharedTypes.Snippet("ac"),
			},
		},
		{
			name: "i-d a-larger end",
			args: args{
				a: sharedTypes.Component{
					Position:  1,
					Insertion: sharedTypes.Snippet("abc"),
				},
				b: sharedTypes.Component{
					Position: 3,
					Deletion: sharedTypes.Snippet("c"),
				},
			},
			want: true,
			wantC: sharedTypes.Component{
				Position:  1,
				Insertion: sharedTypes.Snippet("ab"),
			},
		},
		{
			name: "i-d b-larger start",
			args: args{
				a: sharedTypes.Component{
					Position:  1,
					Insertion: sharedTypes.Snippet("a"),
				},
				b: sharedTypes.Component{
					Position: 1,
					Deletion: sharedTypes.Snippet("abc"),
				},
			},
			want: true,
			wantC: sharedTypes.Component{
				Position: 1,
				Deletion: sharedTypes.Snippet("bc"),
			},
		},
		{
			name: "i-d b-larger center",
			args: args{
				a: sharedTypes.Component{
					Position:  2,
					Insertion: sharedTypes.Snippet("b"),
				},
				b: sharedTypes.Component{
					Position: 1,
					Deletion: sharedTypes.Snippet("abc"),
				},
			},
			want: true,
			wantC: sharedTypes.Component{
				Position: 1,
				Deletion: sharedTypes.Snippet("ac"),
			},
		},
		{
			name: "i-d b-larger end",
			args: args{
				a: sharedTypes.Component{
					Position:  3,
					Insertion: sharedTypes.Snippet("c"),
				},
				b: sharedTypes.Component{
					Position: 1,
					Deletion: sharedTypes.Snippet("abc"),
				},
			},
			want: true,
			wantC: sharedTypes.Component{
				Position: 1,
				Deletion: sharedTypes.Snippet("ab"),
			},
		},
		{
			name: "i-d mismatch",
			args: args{
				a: sharedTypes.Component{
					Position:  4,
					Insertion: sharedTypes.Snippet("d"),
				},
				b: sharedTypes.Component{
					Position: 1,
					Deletion: sharedTypes.Snippet("abc"),
				},
			},
			want: false,
			wantC: sharedTypes.Component{
				Position:  4,
				Insertion: sharedTypes.Snippet("d"),
			},
		},

		{
			name: "d-d append",
			args: args{
				a: sharedTypes.Component{
					Position: 1,
					Deletion: sharedTypes.Snippet("a"),
				},
				b: sharedTypes.Component{
					Position: 1,
					Deletion: sharedTypes.Snippet("b"),
				},
			},
			want: true,
			wantC: sharedTypes.Component{
				Position: 1,
				Deletion: sharedTypes.Snippet("ab"),
			},
		},
		{
			name: "d-d prepend",
			args: args{
				a: sharedTypes.Component{
					Position: 1,
					Deletion: sharedTypes.Snippet("a"),
				},
				b: sharedTypes.Component{
					Position: 0,
					Deletion: sharedTypes.Snippet("b"),
				},
			},
			want: true,
			wantC: sharedTypes.Component{
				Position: 0,
				Deletion: sharedTypes.Snippet("ba"),
			},
		},
		{
			name: "d-d insert b",
			args: args{
				a: sharedTypes.Component{
					Position: 1,
					Deletion: sharedTypes.Snippet("b"),
				},
				b: sharedTypes.Component{
					Position: 0,
					Deletion: sharedTypes.Snippet("ac"),
				},
			},
			want: true,
			wantC: sharedTypes.Component{
				Position: 0,
				Deletion: sharedTypes.Snippet("abc"),
			},
		},
		{
			name: "d-d split",
			args: args{
				a: sharedTypes.Component{
					Position: 1,
					Deletion: sharedTypes.Snippet("a"),
				},
				b: sharedTypes.Component{
					Position: 2,
					Deletion: sharedTypes.Snippet("b"),
				},
			},
			want: false,
			wantC: sharedTypes.Component{
				Position: 1,
				Deletion: sharedTypes.Snippet("a"),
			},
		},
		{
			name: "d-i revert",
			args: args{
				a: sharedTypes.Component{
					Position: 1,
					Deletion: sharedTypes.Snippet("a"),
				},
				b: sharedTypes.Component{
					Position:  1,
					Insertion: sharedTypes.Snippet("a"),
				},
			},
			want: true,
			wantC: sharedTypes.Component{
				Position: 1,
				Deletion: sharedTypes.Snippet(""),
			},
		},
		{
			name: "d-i a-larger start",
			args: args{
				a: sharedTypes.Component{
					Position: 1,
					Deletion: sharedTypes.Snippet("abc"),
				},
				b: sharedTypes.Component{
					Position:  1,
					Insertion: sharedTypes.Snippet("a"),
				},
			},
			want: true,
			wantC: sharedTypes.Component{
				Position: 2,
				Deletion: sharedTypes.Snippet("bc"),
			},
		},
		{
			name: "d-i a-larger center",
			args: args{
				a: sharedTypes.Component{
					Position: 1,
					Deletion: sharedTypes.Snippet("abc"),
				},
				b: sharedTypes.Component{
					Position:  1,
					Insertion: sharedTypes.Snippet("b"),
				},
			},
			// NOTE: diff-match-patch could enable this case
			want: false,
			wantC: sharedTypes.Component{
				Position: 1,
				Deletion: sharedTypes.Snippet("abc"),
			},
		},
		{
			name: "d-i a-larger end",
			args: args{
				a: sharedTypes.Component{
					Position: 1,
					Deletion: sharedTypes.Snippet("abc"),
				},
				b: sharedTypes.Component{
					Position:  3,
					Insertion: sharedTypes.Snippet("c"),
				},
			},
			want: true,
			wantC: sharedTypes.Component{
				Position: 1,
				Deletion: sharedTypes.Snippet("ab"),
			},
		},
		{
			name: "d-i b-larger start",
			args: args{
				a: sharedTypes.Component{
					Position: 1,
					Deletion: sharedTypes.Snippet("a"),
				},
				b: sharedTypes.Component{
					Position:  1,
					Insertion: sharedTypes.Snippet("abc"),
				},
			},
			want: true,
			wantC: sharedTypes.Component{
				Position:  1,
				Insertion: sharedTypes.Snippet("bc"),
			},
		},
		{
			name: "d-i b-larger center",
			args: args{
				a: sharedTypes.Component{
					Position: 1,
					Deletion: sharedTypes.Snippet("b"),
				},
				b: sharedTypes.Component{
					Position:  1,
					Insertion: sharedTypes.Snippet("abc"),
				},
			},
			// NOTE: diff-match-patch could enable this case
			want: false,
			wantC: sharedTypes.Component{
				Position: 1,
				Deletion: sharedTypes.Snippet("b"),
			},
		},
		{
			name: "d-i b-larger end",
			args: args{
				a: sharedTypes.Component{
					Position: 1,
					Deletion: sharedTypes.Snippet("c"),
				},
				b: sharedTypes.Component{
					Position:  1,
					Insertion: sharedTypes.Snippet("abc"),
				},
			},
			want: true,
			wantC: sharedTypes.Component{
				Position:  1,
				Insertion: sharedTypes.Snippet("ab"),
			},
		},
		{
			name: "d-i mismatch",
			args: args{
				a: sharedTypes.Component{
					Position: 1,
					Deletion: sharedTypes.Snippet("x"),
				},
				b: sharedTypes.Component{
					Position:  1,
					Insertion: sharedTypes.Snippet("abc"),
				},
			},
			want: false,
			wantC: sharedTypes.Component{
				Position: 1,
				Deletion: sharedTypes.Snippet("x"),
			},
		},

		{
			name: "i-n",
			args: args{
				a: sharedTypes.Component{
					Position:  1,
					Insertion: sharedTypes.Snippet("a"),
				},
				b: sharedTypes.Component{
					Position: 2,
				},
			},
			want: true,
			wantC: sharedTypes.Component{
				Position:  1,
				Insertion: sharedTypes.Snippet("a"),
			},
		},
		{
			name: "d-n",
			args: args{
				a: sharedTypes.Component{
					Position: 1,
					Deletion: sharedTypes.Snippet("a"),
				},
				b: sharedTypes.Component{
					Position: 2,
				},
			},
			want: true,
			wantC: sharedTypes.Component{
				Position: 1,
				Deletion: sharedTypes.Snippet("a"),
			},
		},
		{
			name: "n-n",
			args: args{
				a: sharedTypes.Component{
					Position: 1,
				},
				b: sharedTypes.Component{
					Position: 2,
				},
			},
			want: true,
			wantC: sharedTypes.Component{
				Position: 2,
			},
		},
		{
			name: "n-i",
			args: args{
				a: sharedTypes.Component{
					Position: 1,
				},
				b: sharedTypes.Component{
					Position:  2,
					Insertion: sharedTypes.Snippet("a"),
				},
			},
			want: true,
			wantC: sharedTypes.Component{
				Position:  2,
				Insertion: sharedTypes.Snippet("a"),
			},
		},
		{
			name: "n-d",
			args: args{
				a: sharedTypes.Component{
					Position: 1,
				},
				b: sharedTypes.Component{
					Position: 2,
					Deletion: sharedTypes.Snippet("a"),
				},
			},
			want: true,
			wantC: sharedTypes.Component{
				Position: 2,
				Deletion: sharedTypes.Snippet("a"),
			},
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			if got, c := mergeComponents(tt.args.a, tt.args.b); got != tt.want {
				t.Errorf("mergeComponents() = %v, want %v", got, tt.want)
			} else if !reflect.DeepEqual(c, tt.wantC) {
				t.Errorf("mergeComponents() = %v, wantC %v", c, tt.wantC)
			}
		})
	}
}
