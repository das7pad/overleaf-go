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
	"testing"

	"github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
)

func TestApply(t *testing.T) {
	type args struct {
		snapshot types.Snapshot
		ops      types.Op
	}
	tests := []struct {
		name    string
		args    args
		want    types.Snapshot
		wantErr bool
	}{
		{
			name: "new",
			args: args{
				snapshot: types.Snapshot(""),
				ops: types.Op{
					{Insertion: types.Snippet("foo"), Position: 0},
				},
			},
			wantErr: false,
			want:    types.Snapshot("foo"),
		},
		{
			name: "append",
			args: args{
				snapshot: types.Snapshot("foo"),
				ops: types.Op{
					{Insertion: types.Snippet("Bar"), Position: 3},
				},
			},
			wantErr: false,
			want:    types.Snapshot("fooBar"),
		},
		{
			name: "insert",
			args: args{
				snapshot: types.Snapshot("fooBaz"),
				ops: types.Op{
					{Insertion: types.Snippet("Bar"), Position: 3},
				},
			},
			wantErr: false,
			want:    types.Snapshot("fooBarBaz"),
		},
		{
			name: "insertUTF-8",
			args: args{
				snapshot: types.Snapshot("fööBaz"),
				ops: types.Op{
					{Insertion: types.Snippet("Bär"), Position: 3},
				},
			},
			wantErr: false,
			want:    types.Snapshot("fööBärBaz"),
		},
		{
			name: "delete",
			args: args{
				snapshot: types.Snapshot("fooBar"),
				ops: types.Op{
					{Deletion: types.Snippet("Bar"), Position: 3},
				},
			},
			wantErr: false,
			want:    types.Snapshot("foo"),
		},
		{
			name: "deleteUTF-8",
			args: args{
				snapshot: types.Snapshot("fööBär"),
				ops: types.Op{
					{Deletion: types.Snippet("Bär"), Position: 3},
				},
			},
			wantErr: false,
			want:    types.Snapshot("föö"),
		},
		{
			name: "deleteAndInsertSequenceS1",
			args: args{
				snapshot: types.Snapshot("fooBaz"),
				ops: types.Op{
					{Insertion: types.Snippet("Bar"), Position: 3},
				},
			},
			wantErr: false,
			want:    types.Snapshot("fooBarBaz"),
		},
		{
			name: "deleteAndInsertSequenceS2",
			args: args{
				snapshot: types.Snapshot("fooBaz"),
				ops: types.Op{
					{Insertion: types.Snippet("Bar"), Position: 3},
					{Deletion: types.Snippet("foo"), Position: 0},
				},
			},
			wantErr: false,
			want:    types.Snapshot("BarBaz"),
		},
		{
			name: "deleteAndInsertSequence",
			args: args{
				snapshot: types.Snapshot("fooBaz"),
				ops: types.Op{
					{Insertion: types.Snippet("Bar"), Position: 3},
					{Deletion: types.Snippet("foo"), Position: 0},
					{Deletion: types.Snippet("Baz"), Position: 3},
				},
			},
			wantErr: false,
			want:    types.Snapshot("Bar"),
		},
		{
			name: "deleteMismatch",
			args: args{
				snapshot: types.Snapshot("fooBar"),
				ops: types.Op{
					{Deletion: types.Snippet("bar"), Position: 3},
				},
			},
			wantErr: true,
			want:    types.Snapshot(""),
		},
		{
			name: "deleteOOB",
			args: args{
				snapshot: types.Snapshot("fooBar"),
				ops: types.Op{
					{Deletion: types.Snippet("barBaz"), Position: 3},
				},
			},
			wantErr: true,
			want:    types.Snapshot(""),
		},
		{
			name: "deleteOOBStart",
			args: args{
				snapshot: types.Snapshot("fooBar"),
				ops: types.Op{
					{Deletion: types.Snippet("barBaz"), Position: 42},
				},
			},
			wantErr: true,
			want:    types.Snapshot(""),
		},
		{
			name: "comment",
			args: args{
				snapshot: types.Snapshot("fooBar"),
				ops: types.Op{
					{Comment: types.Snippet("Bar"), Position: 3},
				},
			},
			wantErr: false,
			want:    types.Snapshot("fooBar"),
		},
		{
			name: "commentUTF-8",
			args: args{
				snapshot: types.Snapshot("fööBär"),
				ops: types.Op{
					{Comment: types.Snippet("Bär"), Position: 3},
				},
			},
			wantErr: false,
			want:    types.Snapshot("fööBär"),
		},
		{
			name: "commentMismatch",
			args: args{
				snapshot: types.Snapshot("fooBar"),
				ops: types.Op{
					{Comment: types.Snippet("bar"), Position: 3},
				},
			},
			wantErr: true,
			want:    types.Snapshot(""),
		},
		{
			name: "commentOOB",
			args: args{
				snapshot: types.Snapshot("fooBar"),
				ops: types.Op{
					{Comment: types.Snippet("out-of-bound"), Position: 3},
				},
			},
			wantErr: true,
			want:    types.Snapshot(""),
		},
		{
			name: "commentOOBStart",
			args: args{
				snapshot: types.Snapshot("fooBar"),
				ops: types.Op{
					{Comment: types.Snippet("out-of-bound"), Position: 42},
				},
			},
			wantErr: true,
			want:    types.Snapshot(""),
		},
		{
			name: "deleteAndRestoreReuseSlice",
			args: args{
				snapshot: types.Snapshot("fooBar"),
				ops: types.Op{
					{Deletion: types.Snippet("Bar"), Position: 3},
					{Insertion: types.Snippet("Bar"), Position: 3},
				},
			},
			wantErr: false,
			want:    types.Snapshot("fooBar"),
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
