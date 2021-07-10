// Golang port of the Overleaf document-updater service
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

package text

import (
	"reflect"
	"testing"

	"github.com/das7pad/document-updater/pkg/types"
)

func TestDiff(t *testing.T) {
	type args struct {
		before types.Snapshot
		after  types.Snapshot
	}
	tests := []struct {
		name string
		args args
		want types.Op
	}{
		{
			name: "insertNewText",
			args: args{
				before: types.Snapshot("hello world"),
				after:  types.Snapshot("hello beautiful world"),
			},
			want: types.Op{
				{
					Insertion: types.Snippet("beautiful "),
					Position:  6,
				},
			},
		},
		{
			name: "insertNewTextUTF-8",
			args: args{
				before: types.Snapshot("hellö wörld"),
				after:  types.Snapshot("hellö beäütifül wörld"),
			},
			want: types.Op{
				{
					Insertion: types.Snippet(" beäütifül"),
					Position:  5,
				},
			},
		},
		{
			name: "shiftInsertAfterInsert",
			args: args{
				before: types.Snapshot("the boy played with the ball"),
				after:  types.Snapshot("the tall boy played with the red ball"),
			},
			want: types.Op{
				{
					Insertion: types.Snippet("tall "),
					Position:  4,
				},
				{
					Insertion: types.Snippet("red "),
					Position:  29,
				},
			},
		},
		{
			name: "delete",
			args: args{
				before: types.Snapshot("hello beautiful world"),
				after:  types.Snapshot("hello world"),
			},
			want: types.Op{
				{
					Deletion: types.Snippet("beautiful "),
					Position: 6,
				},
			},
		},
		{
			name: "shiftDeleteAfterDelete",
			args: args{
				before: types.Snapshot("the tall boy played with the red ball"),
				after:  types.Snapshot("the boy played with the ball"),
			},
			want: types.Op{
				{
					Deletion: types.Snippet("tall "),
					Position: 4,
				},
				{
					Deletion: types.Snippet("red "),
					Position: 24,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Diff(tt.args.before, tt.args.after); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Diff() = %v, want %v", got, tt.want)
			}
		})
	}
}
