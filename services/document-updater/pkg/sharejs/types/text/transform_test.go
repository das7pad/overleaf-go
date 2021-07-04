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

func TestTransform(t *testing.T) {
	type args struct {
		op      types.Op
		otherOp types.Op
	}
	tests := []struct {
		name    string
		args    args
		want    types.Op
		wantErr bool
	}{
		{
			name: "emptyOther",
			args: args{
				op: types.Op{
					{Insertion: "foo", Position: 0},
				},
				otherOp: nil,
			},
			want: types.Op{
				{Insertion: "foo", Position: 0},
			},
			wantErr: false,
		},
		{
			name: "insertionPassThroughDeletion",
			args: args{
				op: types.Op{
					{Insertion: "foo", Position: 10},
				},
				otherOp: types.Op{
					{Deletion: "foo", Position: 42},
				},
			},
			want: types.Op{
				{Insertion: "foo", Position: 10},
			},
			wantErr: false,
		},
		{
			name: "insertionPassThroughInsertion",
			args: args{
				op: types.Op{
					{Insertion: "foo", Position: 10},
				},
				otherOp: types.Op{
					{Insertion: "foo", Position: 42},
				},
			},
			want: types.Op{
				{Insertion: "foo", Position: 10},
			},
			wantErr: false,
		},
		{
			name: "insertionPassThroughComment",
			args: args{
				op: types.Op{
					{Insertion: "foo", Position: 10},
				},
				otherOp: types.Op{
					{Comment: "foo", Position: 42},
				},
			},
			want: types.Op{
				{Insertion: "foo", Position: 10},
			},
			wantErr: false,
		},
		{
			name: "deletionPassThroughInsertion",
			args: args{
				op: types.Op{
					{Deletion: "foo", Position: 10},
				},
				otherOp: types.Op{
					{Insertion: "foo", Position: 42},
				},
			},
			want: types.Op{
				{Deletion: "foo", Position: 10},
			},
			wantErr: false,
		},
		{
			name: "deletionPassThroughDeletion",
			args: args{
				op: types.Op{
					{Deletion: "foo", Position: 10},
				},
				otherOp: types.Op{
					{Deletion: "foo", Position: 42},
				},
			},
			want: types.Op{
				{Deletion: "foo", Position: 10},
			},
			wantErr: false,
		},
		{
			name: "deletionPassThroughComment",
			args: args{
				op: types.Op{
					{Deletion: "foo", Position: 10},
				},
				otherOp: types.Op{
					{Comment: "foo", Position: 42},
				},
			},
			want: types.Op{
				{Deletion: "foo", Position: 10},
			},
			wantErr: false,
		},
		{
			name: "mergeDeletions",
			args: args{
				op: types.Op{
					{Deletion: "foo", Position: 0},
					{Deletion: "Baz", Position: 0},
				},
				otherOp: types.Op{
					{Comment: "do-not-matter", Position: 42},
				},
			},
			want: types.Op{
				{Deletion: "fooBaz", Position: 0},
			},
			wantErr: false,
		},
		{
			name: "mergeInsertions",
			args: args{
				op: types.Op{
					{Insertion: "foo", Position: 0},
					{Insertion: "Baz", Position: 3},
				},
				otherOp: types.Op{
					{Comment: "do-not-matter", Position: 42},
				},
			},
			want: types.Op{
				{Insertion: "fooBaz", Position: 0},
			},
			wantErr: false,
		},
		{
			name: "splitDeletionInsertion",
			args: args{
				op: types.Op{
					{Deletion: "fooBaz", Position: 0},
				},
				otherOp: types.Op{
					{Insertion: "Bar", Position: 3},
				},
			},
			want: types.Op{
				{Deletion: "foo", Position: 0},
				{Deletion: "Baz", Position: 3},
			},
			wantErr: false,
		},
		{
			name: "splitDeletionInsertionMulti",
			args: args{
				op: types.Op{
					{Deletion: "fooBaz", Position: 0},
				},
				otherOp: types.Op{
					{Insertion: "Bar", Position: 3},
					{Comment: "foo", Position: 42},
				},
			},
			want: types.Op{
				{Deletion: "foo", Position: 0},
				{Deletion: "Baz", Position: 3},
			},
			wantErr: false,
		},
		{
			name: "splitDeletionInsertionMultiRev",
			args: args{
				op: types.Op{
					{Insertion: "Bar", Position: 3},
					{Comment: "foo", Position: 42},
				},
				otherOp: types.Op{
					{Deletion: "fooBaz", Position: 0},
				},
			},
			want: types.Op{
				{Insertion: "Bar", Position: 0},
				{Comment: "foo", Position: 36},
			},
			wantErr: false,
		},
		{
			name: "splitDeletionInsertionMultiRevMismatch",
			args: args{
				op: types.Op{
					{Insertion: "Bar", Position: 3},
					{Comment: "foo", Position: 6},
				},
				otherOp: types.Op{
					{Deletion: "fooBaz", Position: 0},
				},
			},
			wantErr: true,
		},
		{
			name: "shiftInsertionFromDeletion",
			args: args{
				op: types.Op{
					{Insertion: "foo", Position: 10},
				},
				otherOp: types.Op{
					{Deletion: "foo", Position: 3},
				},
			},
			want: types.Op{
				{Insertion: "foo", Position: 7},
			},
			wantErr: false,
		},
		{
			name: "shiftInsertionFromInsertion",
			args: args{
				op: types.Op{
					{Insertion: "foo", Position: 10},
				},
				otherOp: types.Op{
					{Insertion: "foo", Position: 3},
				},
			},
			want: types.Op{
				{Insertion: "foo", Position: 13},
			},
			wantErr: false,
		},
		{
			name: "shiftDeletionFromInsertion",
			args: args{
				op: types.Op{
					{Deletion: "foo", Position: 10},
				},
				otherOp: types.Op{
					{Insertion: "foo", Position: 3},
				},
			},
			want: types.Op{
				{Deletion: "foo", Position: 13},
			},
			wantErr: false,
		},
		{
			name: "shiftDeletionFromDeletion",
			args: args{
				op: types.Op{
					{Deletion: "foo", Position: 10},
				},
				otherOp: types.Op{
					{Deletion: "foo", Position: 3},
				},
			},
			want: types.Op{
				{Deletion: "foo", Position: 7},
			},
			wantErr: false,
		},
		{
			name: "shiftCommentFromInsertion",
			args: args{
				op: types.Op{
					{Comment: "foo", Position: 10},
				},
				otherOp: types.Op{
					{Insertion: "foo", Position: 3},
				},
			},
			want: types.Op{
				{Comment: "foo", Position: 13},
			},
			wantErr: false,
		},
		{
			name: "eatDeletionChild",
			args: args{
				op: types.Op{
					{Deletion: "foo", Position: 0},
				},
				otherOp: types.Op{
					{Deletion: "fooBar", Position: 0},
				},
			},
			want:    types.Op{},
			wantErr: false,
		},
		{
			name: "eatDeletionChildInner",
			args: args{
				op: types.Op{
					{Deletion: "oo", Position: 1},
				},
				otherOp: types.Op{
					{Deletion: "fooBar", Position: 0},
				},
			},
			want:    types.Op{},
			wantErr: false,
		},
		{
			name: "eatDeletionChildInnerMulti",
			args: args{
				op: types.Op{
					{Deletion: "oo", Position: 1},
				},
				otherOp: types.Op{
					{Deletion: "foo", Position: 0},
					{Deletion: "Bar", Position: 0},
				},
			},
			want:    types.Op{},
			wantErr: false,
		},
		{
			name: "eatDeletionChildInnerMultiRev",
			args: args{
				op: types.Op{
					{Deletion: "oo", Position: 1},
					{Deletion: "Bar", Position: 1},
					{Comment: "foo", Position: 42},
				},
				otherOp: types.Op{
					{Deletion: "foo", Position: 0},
					{Deletion: "Bar", Position: 0},
				},
			},
			want: types.Op{
				{Comment: "foo", Position: 41}},
			wantErr: false,
		},
		{
			name: "eatDeletionChildPartialStart",
			args: args{
				op: types.Op{
					{Deletion: "foo", Position: 0},
				},
				otherOp: types.Op{
					{Deletion: "ooBar", Position: 1},
				},
			},
			want: types.Op{
				{Deletion: "f", Position: 0},
			},
			wantErr: false,
		},
		{
			name: "eatDeletionChildPartialEnd",
			args: args{
				op: types.Op{
					{Deletion: "Bar", Position: 3},
				},
				otherOp: types.Op{
					{Deletion: "ooBa", Position: 1},
				},
			},
			want: types.Op{
				{Deletion: "r", Position: 1},
			},
			wantErr: false,
		},
		{
			name: "deletionMismatch",
			args: args{
				op: types.Op{
					{Deletion: "Bar", Position: 0},
				},
				otherOp: types.Op{
					{Deletion: "foo", Position: 0},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "commentAndDeletionMismatch",
			args: args{
				op: types.Op{
					{Comment: "Bar", Position: 0},
				},
				otherOp: types.Op{
					{Deletion: "foo", Position: 0},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "commentAndDeletionMismatchMulti",
			args: args{
				op: types.Op{
					{Comment: "foo", Position: 0},
					{Comment: "Bar", Position: 42},
				},
				otherOp: types.Op{
					{Deletion: "Baz", Position: 0},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "commentAndDeletionMismatchMultiReverse",
			args: args{
				op: types.Op{
					{Deletion: "foo", Position: 0},
					{Comment: "Bar", Position: 42},
				},
				otherOp: types.Op{
					{Comment: "Baz", Position: 0},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "commentCutFromDeletion",
			args: args{
				op: types.Op{
					{Comment: "fooBar", Position: 0},
				},
				otherOp: types.Op{
					{Deletion: "foo", Position: 0},
				},
			},
			want: types.Op{
				{Comment: "Bar", Position: 0},
			},
			wantErr: false,
		},
		{
			name: "commentCutFromDeletionPartial",
			args: args{
				op: types.Op{
					{Comment: "fooBarBaz", Position: 0},
				},
				otherOp: types.Op{
					{Deletion: "Bar", Position: 3},
				},
			},
			want: types.Op{
				{Comment: "fooBaz", Position: 0},
			},
			wantErr: false,
		},
		{
			name: "commentPassThroughDeletion",
			args: args{
				op: types.Op{
					{Comment: "fooBar", Position: 0},
				},
				otherOp: types.Op{
					{Deletion: "Bar", Position: 42},
				},
			},
			want: types.Op{
				{Comment: "fooBar", Position: 0},
			},
			wantErr: false,
		},
		{
			name: "commentPassThroughComment",
			args: args{
				op: types.Op{
					{Comment: "fooBar", Position: 0},
				},
				otherOp: types.Op{
					{Comment: "Bar", Position: 42},
				},
			},
			want: types.Op{
				{Comment: "fooBar", Position: 0},
			},
			wantErr: false,
		},
		{
			name: "commentExtendedByInsertion",
			args: args{
				op: types.Op{
					{Comment: "fooBaz", Position: 0},
				},
				otherOp: types.Op{
					{Insertion: "Bar", Position: 3},
				},
			},
			want: types.Op{
				{Comment: "fooBarBaz", Position: 0},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Transform(tt.args.op, tt.args.otherOp)
			if (err != nil) != tt.wantErr {
				t.Errorf("Transform() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Transform() got = %v, want %v", got, tt.want)
			}
		})
	}
}
