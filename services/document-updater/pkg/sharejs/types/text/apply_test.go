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

package text

import (
	"testing"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func TestApply(t *testing.T) {
	type args struct {
		snapshot sharedTypes.Snapshot
		ops      sharedTypes.Op
	}
	tests := []struct {
		name    string
		args    args
		want    sharedTypes.Snapshot
		wantErr bool
	}{
		{
			name: "new",
			args: args{
				snapshot: sharedTypes.Snapshot(""),
				ops: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("foo"), Position: 0},
				},
			},
			wantErr: false,
			want:    sharedTypes.Snapshot("foo"),
		},
		{
			name: "append",
			args: args{
				snapshot: sharedTypes.Snapshot("foo"),
				ops: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("Bar"), Position: 3},
				},
			},
			wantErr: false,
			want:    sharedTypes.Snapshot("fooBar"),
		},
		{
			name: "insert",
			args: args{
				snapshot: sharedTypes.Snapshot("fooBaz"),
				ops: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("Bar"), Position: 3},
				},
			},
			wantErr: false,
			want:    sharedTypes.Snapshot("fooBarBaz"),
		},
		{
			name: "insertUTF-8",
			args: args{
				snapshot: sharedTypes.Snapshot("fööBaz"),
				ops: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("Bär"), Position: 3},
				},
			},
			wantErr: false,
			want:    sharedTypes.Snapshot("fööBärBaz"),
		},
		{
			name: "delete",
			args: args{
				snapshot: sharedTypes.Snapshot("fooBar"),
				ops: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("Bar"), Position: 3},
				},
			},
			wantErr: false,
			want:    sharedTypes.Snapshot("foo"),
		},
		{
			name: "deleteUTF-8",
			args: args{
				snapshot: sharedTypes.Snapshot("fööBär"),
				ops: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("Bär"), Position: 3},
				},
			},
			wantErr: false,
			want:    sharedTypes.Snapshot("föö"),
		},
		{
			name: "deleteAndInsertSequenceS1",
			args: args{
				snapshot: sharedTypes.Snapshot("fooBaz"),
				ops: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("Bar"), Position: 3},
				},
			},
			wantErr: false,
			want:    sharedTypes.Snapshot("fooBarBaz"),
		},
		{
			name: "deleteAndInsertSequenceS2",
			args: args{
				snapshot: sharedTypes.Snapshot("fooBaz"),
				ops: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("Bar"), Position: 3},
					{Deletion: sharedTypes.Snippet("foo"), Position: 0},
				},
			},
			wantErr: false,
			want:    sharedTypes.Snapshot("BarBaz"),
		},
		{
			name: "deleteAndInsertSequence",
			args: args{
				snapshot: sharedTypes.Snapshot("fooBaz"),
				ops: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("Bar"), Position: 3},
					{Deletion: sharedTypes.Snippet("foo"), Position: 0},
					{Deletion: sharedTypes.Snippet("Baz"), Position: 3},
				},
			},
			wantErr: false,
			want:    sharedTypes.Snapshot("Bar"),
		},
		{
			name: "deleteMismatch",
			args: args{
				snapshot: sharedTypes.Snapshot("fooBar"),
				ops: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("bar"), Position: 3},
				},
			},
			wantErr: true,
			want:    sharedTypes.Snapshot(""),
		},
		{
			name: "deleteOOB",
			args: args{
				snapshot: sharedTypes.Snapshot("fooBar"),
				ops: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("barBaz"), Position: 3},
				},
			},
			wantErr: true,
			want:    sharedTypes.Snapshot(""),
		},
		{
			name: "deleteOOBStart",
			args: args{
				snapshot: sharedTypes.Snapshot("fooBar"),
				ops: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("barBaz"), Position: 42},
				},
			},
			wantErr: true,
			want:    sharedTypes.Snapshot(""),
		},
		{
			name: "comment",
			args: args{
				snapshot: sharedTypes.Snapshot("fooBar"),
				ops: sharedTypes.Op{
					{Comment: sharedTypes.Snippet("Bar"), Position: 3},
				},
			},
			wantErr: false,
			want:    sharedTypes.Snapshot("fooBar"),
		},
		{
			name: "commentUTF-8",
			args: args{
				snapshot: sharedTypes.Snapshot("fööBär"),
				ops: sharedTypes.Op{
					{Comment: sharedTypes.Snippet("Bär"), Position: 3},
				},
			},
			wantErr: false,
			want:    sharedTypes.Snapshot("fööBär"),
		},
		{
			name: "commentMismatch",
			args: args{
				snapshot: sharedTypes.Snapshot("fooBar"),
				ops: sharedTypes.Op{
					{Comment: sharedTypes.Snippet("bar"), Position: 3},
				},
			},
			wantErr: true,
			want:    sharedTypes.Snapshot(""),
		},
		{
			name: "commentOOB",
			args: args{
				snapshot: sharedTypes.Snapshot("fooBar"),
				ops: sharedTypes.Op{
					{Comment: sharedTypes.Snippet("out-of-bound"), Position: 3},
				},
			},
			wantErr: true,
			want:    sharedTypes.Snapshot(""),
		},
		{
			name: "commentOOBStart",
			args: args{
				snapshot: sharedTypes.Snapshot("fooBar"),
				ops: sharedTypes.Op{
					{Comment: sharedTypes.Snippet("out-of-bound"), Position: 42},
				},
			},
			wantErr: true,
			want:    sharedTypes.Snapshot(""),
		},
		{
			name: "deleteAndRestoreReuseSlice",
			args: args{
				snapshot: sharedTypes.Snapshot("fooBar"),
				ops: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("Bar"), Position: 3},
					{Insertion: sharedTypes.Snippet("Bar"), Position: 3},
				},
			},
			wantErr: false,
			want:    sharedTypes.Snapshot("fooBar"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Apply(tt.args.snapshot, tt.args.ops)
			if (err != nil) != tt.wantErr {
				t.Errorf("Apply() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if string(got) != string(tt.want) {
				t.Errorf("Apply() got = %v, want %v", string(got), string(tt.want))
			}
		})
	}
}
