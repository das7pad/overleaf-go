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

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

//goland:noinspection SpellCheckingInspection
func TestDiff(t *testing.T) {
	type args struct {
		before sharedTypes.Snapshot
		after  sharedTypes.Snapshot
	}
	tests := []struct {
		name string
		args args
		want sharedTypes.Op
	}{
		{
			name: "insertNewText",
			args: args{
				before: sharedTypes.Snapshot("hello world"),
				after:  sharedTypes.Snapshot("hello beautiful world"),
			},
			want: sharedTypes.Op{
				{
					Insertion: sharedTypes.Snippet("beautiful "),
					Position:  6,
				},
			},
		},
		{
			name: "insertNewTextUTF-8",
			args: args{
				before: sharedTypes.Snapshot("hellö wörld"),
				after:  sharedTypes.Snapshot("hellö beäütifül wörld"),
			},
			want: sharedTypes.Op{
				{
					Insertion: sharedTypes.Snippet(" beäütifül"),
					Position:  5,
				},
			},
		},
		{
			name: "shiftInsertAfterInsert",
			args: args{
				before: sharedTypes.Snapshot("the boy played with the ball"),
				after:  sharedTypes.Snapshot("the tall boy played with the red ball"),
			},
			want: sharedTypes.Op{
				{
					Insertion: sharedTypes.Snippet("tall "),
					Position:  4,
				},
				{
					Insertion: sharedTypes.Snippet("red "),
					Position:  29,
				},
			},
		},
		{
			name: "delete",
			args: args{
				before: sharedTypes.Snapshot("hello beautiful world"),
				after:  sharedTypes.Snapshot("hello world"),
			},
			want: sharedTypes.Op{
				{
					Deletion: sharedTypes.Snippet("beautiful "),
					Position: 6,
				},
			},
		},
		{
			name: "shiftDeleteAfterDelete",
			args: args{
				before: sharedTypes.Snapshot("the tall boy played with the red ball"),
				after:  sharedTypes.Snapshot("the boy played with the ball"),
			},
			want: sharedTypes.Op{
				{
					Deletion: sharedTypes.Snippet("tall "),
					Position: 4,
				},
				{
					Deletion: sharedTypes.Snippet("red "),
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
