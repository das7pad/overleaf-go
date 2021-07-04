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

	"github.com/das7pad/document-updater/pkg/types"
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
				snapshot: "",
				ops: types.Op{
					{Insertion: "foo", Position: 0},
				},
			},
			wantErr: false,
			want:    "foo",
		},
		{
			name: "append",
			args: args{
				snapshot: "foo",
				ops: types.Op{
					{Insertion: "Bar", Position: 3},
				},
			},
			wantErr: false,
			want:    "fooBar",
		},
		{
			name: "insert",
			args: args{
				snapshot: "fooBaz",
				ops: types.Op{
					{Insertion: "Bar", Position: 3},
				},
			},
			wantErr: false,
			want:    "fooBarBaz",
		},
		{
			name: "delete",
			args: args{
				snapshot: "fooBar",
				ops: types.Op{
					{Deletion: "Bar", Position: 3},
				},
			},
			wantErr: false,
			want:    "foo",
		},
		{
			name: "deleteAndInsertSequence",
			args: args{
				snapshot: "fooBaz",
				ops: types.Op{
					{Insertion: "Bar", Position: 3},
					{Deletion: "foo", Position: 0},
					{Deletion: "Baz", Position: 3},
				},
			},
			wantErr: false,
			want:    "Bar",
		},
		{
			name: "deleteMismatch",
			args: args{
				snapshot: "fooBar",
				ops: types.Op{
					{Deletion: "bar", Position: 3},
				},
			},
			wantErr: true,
			want:    "",
		},
		{
			name: "deleteOOB",
			args: args{
				snapshot: "fooBar",
				ops: types.Op{
					{Deletion: "barBaz", Position: 3},
				},
			},
			wantErr: true,
			want:    "",
		},
		{
			name: "deleteOOBStart",
			args: args{
				snapshot: "fooBar",
				ops: types.Op{
					{Deletion: "barBaz", Position: 42},
				},
			},
			wantErr: true,
			want:    "",
		},
		{
			name: "comment",
			args: args{
				snapshot: "fooBar",
				ops: types.Op{
					{Comment: "Bar", Position: 3},
				},
			},
			wantErr: false,
			want:    "fooBar",
		},
		{
			name: "commentMismatch",
			args: args{
				snapshot: "fooBar",
				ops: types.Op{
					{Comment: "bar", Position: 3},
				},
			},
			wantErr: true,
			want:    "",
		},
		{
			name: "commentOOB",
			args: args{
				snapshot: "fooBar",
				ops: types.Op{
					{Comment: "out-of-bound", Position: 3},
				},
			},
			wantErr: true,
			want:    "",
		},
		{
			name: "commentOOBStart",
			args: args{
				snapshot: "fooBar",
				ops: types.Op{
					{Comment: "out-of-bound", Position: 42},
				},
			},
			wantErr: true,
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Apply(tt.args.snapshot, tt.args.ops)
			if (err != nil) != tt.wantErr {
				t.Errorf("Apply() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Apply() got = %v, want %v", got, tt.want)
			}
		})
	}
}
