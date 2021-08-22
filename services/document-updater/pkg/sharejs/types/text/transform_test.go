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

func TestTransform(t *testing.T) {
	type args struct {
		op      sharedTypes.Op
		otherOp sharedTypes.Op
	}
	tests := []struct {
		name    string
		args    args
		want    sharedTypes.Op
		wantErr bool
	}{
		{
			name: "emptyOther",
			args: args{
				op: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("foo"), Position: 0},
				},
				otherOp: nil,
			},
			want: sharedTypes.Op{
				{Insertion: sharedTypes.Snippet("foo"), Position: 0},
			},
			wantErr: false,
		},
		{
			name: "insertionPassThroughDeletion",
			args: args{
				op: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("foo"), Position: 10},
				},
				otherOp: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("foo"), Position: 42},
				},
			},
			want: sharedTypes.Op{
				{Insertion: sharedTypes.Snippet("foo"), Position: 10},
			},
			wantErr: false,
		},
		{
			name: "insertionPassThroughInsertion",
			args: args{
				op: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("foo"), Position: 10},
				},
				otherOp: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("foo"), Position: 42},
				},
			},
			want: sharedTypes.Op{
				{Insertion: sharedTypes.Snippet("foo"), Position: 10},
			},
			wantErr: false,
		},
		{
			name: "insertionPassThroughComment",
			args: args{
				op: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("foo"), Position: 10},
				},
				otherOp: sharedTypes.Op{
					{Comment: sharedTypes.Snippet("foo"), Position: 42},
				},
			},
			want: sharedTypes.Op{
				{Insertion: sharedTypes.Snippet("foo"), Position: 10},
			},
			wantErr: false,
		},
		{
			name: "deletionPassThroughInsertion",
			args: args{
				op: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("foo"), Position: 10},
				},
				otherOp: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("foo"), Position: 42},
				},
			},
			want: sharedTypes.Op{
				{Deletion: sharedTypes.Snippet("foo"), Position: 10},
			},
			wantErr: false,
		},
		{
			name: "deletionPassThroughDeletion",
			args: args{
				op: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("foo"), Position: 10},
				},
				otherOp: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("foo"), Position: 42},
				},
			},
			want: sharedTypes.Op{
				{Deletion: sharedTypes.Snippet("foo"), Position: 10},
			},
			wantErr: false,
		},
		{
			name: "deletionPassThroughComment",
			args: args{
				op: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("foo"), Position: 10},
				},
				otherOp: sharedTypes.Op{
					{Comment: sharedTypes.Snippet("foo"), Position: 42},
				},
			},
			want: sharedTypes.Op{
				{Deletion: sharedTypes.Snippet("foo"), Position: 10},
			},
			wantErr: false,
		},
		{
			name: "mergeDeletions",
			args: args{
				op: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("foo"), Position: 0},
					{Deletion: sharedTypes.Snippet("Baz"), Position: 0},
				},
				otherOp: sharedTypes.Op{
					{Comment: sharedTypes.Snippet("do-not-matter"), Position: 42},
				},
			},
			want: sharedTypes.Op{
				{Deletion: sharedTypes.Snippet("fooBaz"), Position: 0},
			},
			wantErr: false,
		},
		{
			name: "mergeInsertions",
			args: args{
				op: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("foo"), Position: 0},
					{Insertion: sharedTypes.Snippet("Baz"), Position: 3},
				},
				otherOp: sharedTypes.Op{
					{Comment: sharedTypes.Snippet("do-not-matter"), Position: 42},
				},
			},
			want: sharedTypes.Op{
				{Insertion: sharedTypes.Snippet("fooBaz"), Position: 0},
			},
			wantErr: false,
		},
		{
			name: "splitDeletionInsertion",
			args: args{
				op: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("fooBaz"), Position: 0},
				},
				otherOp: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("Bar"), Position: 3},
				},
			},
			want: sharedTypes.Op{
				{Deletion: sharedTypes.Snippet("foo"), Position: 0},
				{Deletion: sharedTypes.Snippet("Baz"), Position: 3},
			},
			wantErr: false,
		},
		{
			name: "splitDeletionInsertionMulti",
			args: args{
				op: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("fooBaz"), Position: 0},
				},
				otherOp: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("Bar"), Position: 3},
					{Comment: sharedTypes.Snippet("foo"), Position: 42},
				},
			},
			want: sharedTypes.Op{
				{Deletion: sharedTypes.Snippet("foo"), Position: 0},
				{Deletion: sharedTypes.Snippet("Baz"), Position: 3},
			},
			wantErr: false,
		},
		{
			name: "splitDeletionInsertionMultiRev",
			args: args{
				op: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("Bar"), Position: 3},
					{Comment: sharedTypes.Snippet("foo"), Position: 42},
				},
				otherOp: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("fooBaz"), Position: 0},
				},
			},
			want: sharedTypes.Op{
				{Insertion: sharedTypes.Snippet("Bar"), Position: 0},
				{Comment: sharedTypes.Snippet("foo"), Position: 36},
			},
			wantErr: false,
		},
		{
			name: "splitDeletionInsertionMultiRevMismatch",
			args: args{
				op: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("Bar"), Position: 3},
					{Comment: sharedTypes.Snippet("foo"), Position: 6},
				},
				otherOp: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("fooBaz"), Position: 0},
				},
			},
			wantErr: true,
		},
		{
			name: "shiftInsertionFromDeletion",
			args: args{
				op: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("foo"), Position: 10},
				},
				otherOp: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("foo"), Position: 3},
				},
			},
			want: sharedTypes.Op{
				{Insertion: sharedTypes.Snippet("foo"), Position: 7},
			},
			wantErr: false,
		},
		{
			name: "shiftInsertionFromInsertion",
			args: args{
				op: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("foo"), Position: 10},
				},
				otherOp: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("foo"), Position: 3},
				},
			},
			want: sharedTypes.Op{
				{Insertion: sharedTypes.Snippet("foo"), Position: 13},
			},
			wantErr: false,
		},
		{
			name: "shiftDeletionFromInsertion",
			args: args{
				op: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("foo"), Position: 10},
				},
				otherOp: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("foo"), Position: 3},
				},
			},
			want: sharedTypes.Op{
				{Deletion: sharedTypes.Snippet("foo"), Position: 13},
			},
			wantErr: false,
		},
		{
			name: "shiftDeletionFromDeletion",
			args: args{
				op: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("foo"), Position: 10},
				},
				otherOp: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("foo"), Position: 3},
				},
			},
			want: sharedTypes.Op{
				{Deletion: sharedTypes.Snippet("foo"), Position: 7},
			},
			wantErr: false,
		},
		{
			name: "shiftCommentFromInsertion",
			args: args{
				op: sharedTypes.Op{
					{Comment: sharedTypes.Snippet("foo"), Position: 10},
				},
				otherOp: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("foo"), Position: 3},
				},
			},
			want: sharedTypes.Op{
				{Comment: sharedTypes.Snippet("foo"), Position: 13},
			},
			wantErr: false,
		},
		{
			name: "shiftInsertionFromDeletionUTF-8",
			args: args{
				op: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("foo"), Position: 10},
				},
				otherOp: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("föö"), Position: 3},
				},
			},
			want: sharedTypes.Op{
				{Insertion: sharedTypes.Snippet("foo"), Position: 7},
			},
			wantErr: false,
		},
		{
			name: "shiftInsertionFromInsertionUTF-8",
			args: args{
				op: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("foo"), Position: 10},
				},
				otherOp: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("föö"), Position: 3},
				},
			},
			want: sharedTypes.Op{
				{Insertion: sharedTypes.Snippet("foo"), Position: 13},
			},
			wantErr: false,
		},
		{
			name: "shiftDeletionFromInsertionUTF-8",
			args: args{
				op: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("foo"), Position: 10},
				},
				otherOp: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("föö"), Position: 3},
				},
			},
			want: sharedTypes.Op{
				{Deletion: sharedTypes.Snippet("foo"), Position: 13},
			},
			wantErr: false,
		},
		{
			name: "shiftDeletionFromDeletionUTF-8",
			args: args{
				op: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("foo"), Position: 10},
				},
				otherOp: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("föö"), Position: 3},
				},
			},
			want: sharedTypes.Op{
				{Deletion: sharedTypes.Snippet("foo"), Position: 7},
			},
			wantErr: false,
		},
		{
			name: "shiftCommentFromInsertionUTF-8",
			args: args{
				op: sharedTypes.Op{
					{Comment: sharedTypes.Snippet("foo"), Position: 10},
				},
				otherOp: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("föö"), Position: 3},
				},
			},
			want: sharedTypes.Op{
				{Comment: sharedTypes.Snippet("foo"), Position: 13},
			},
			wantErr: false,
		},
		{
			name: "eatDeletionChild",
			args: args{
				op: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("foo"), Position: 0},
				},
				otherOp: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("fooBar"), Position: 0},
				},
			},
			want:    sharedTypes.Op{},
			wantErr: false,
		},
		{
			name: "eatDeletionChildInner",
			args: args{
				op: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("oo"), Position: 1},
				},
				otherOp: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("fooBar"), Position: 0},
				},
			},
			want:    sharedTypes.Op{},
			wantErr: false,
		},
		{
			name: "eatDeletionChildInnerMulti",
			args: args{
				op: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("oo"), Position: 1},
				},
				otherOp: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("foo"), Position: 0},
					{Deletion: sharedTypes.Snippet("Bar"), Position: 0},
				},
			},
			want:    sharedTypes.Op{},
			wantErr: false,
		},
		{
			name: "eatDeletionChildInnerMultiRev",
			args: args{
				op: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("oo"), Position: 1},
					{Deletion: sharedTypes.Snippet("Bar"), Position: 1},
					{Comment: sharedTypes.Snippet("foo"), Position: 42},
				},
				otherOp: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("foo"), Position: 0},
					{Deletion: sharedTypes.Snippet("Bar"), Position: 0},
				},
			},
			want: sharedTypes.Op{
				{Comment: sharedTypes.Snippet("foo"), Position: 41}},
			wantErr: false,
		},
		{
			name: "eatDeletionChildPartialStart",
			args: args{
				op: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("foo"), Position: 0},
				},
				otherOp: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("ooBar"), Position: 1},
				},
			},
			want: sharedTypes.Op{
				{Deletion: sharedTypes.Snippet("f"), Position: 0},
			},
			wantErr: false,
		},
		{
			name: "eatDeletionChildPartialEnd",
			args: args{
				op: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("Bar"), Position: 3},
				},
				otherOp: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("ooBa"), Position: 1},
				},
			},
			want: sharedTypes.Op{
				{Deletion: sharedTypes.Snippet("r"), Position: 1},
			},
			wantErr: false,
		},
		{
			name: "deletionMismatch",
			args: args{
				op: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("Bar"), Position: 0},
				},
				otherOp: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("foo"), Position: 0},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "commentAndDeletionMismatch",
			args: args{
				op: sharedTypes.Op{
					{Comment: sharedTypes.Snippet("Bar"), Position: 0},
				},
				otherOp: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("foo"), Position: 0},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "commentAndDeletionMismatchMulti",
			args: args{
				op: sharedTypes.Op{
					{Comment: sharedTypes.Snippet("foo"), Position: 0},
					{Comment: sharedTypes.Snippet("Bar"), Position: 42},
				},
				otherOp: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("Baz"), Position: 0},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "commentAndDeletionMismatchMultiReverse",
			args: args{
				op: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("foo"), Position: 0},
					{Comment: sharedTypes.Snippet("Bar"), Position: 42},
				},
				otherOp: sharedTypes.Op{
					{Comment: sharedTypes.Snippet("Baz"), Position: 0},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "commentCutFromDeletion",
			args: args{
				op: sharedTypes.Op{
					{Comment: sharedTypes.Snippet("fooBar"), Position: 0},
				},
				otherOp: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("foo"), Position: 0},
				},
			},
			want: sharedTypes.Op{
				{Comment: sharedTypes.Snippet("Bar"), Position: 0},
			},
			wantErr: false,
		},
		{
			name: "commentCutFromDeletionPartial",
			args: args{
				op: sharedTypes.Op{
					{Comment: sharedTypes.Snippet("fooBarBaz"), Position: 0},
				},
				otherOp: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("Bar"), Position: 3},
				},
			},
			want: sharedTypes.Op{
				{Comment: sharedTypes.Snippet("fooBaz"), Position: 0},
			},
			wantErr: false,
		},
		{
			name: "commentPassThroughDeletion",
			args: args{
				op: sharedTypes.Op{
					{Comment: sharedTypes.Snippet("fooBar"), Position: 0},
				},
				otherOp: sharedTypes.Op{
					{Deletion: sharedTypes.Snippet("Bar"), Position: 42},
				},
			},
			want: sharedTypes.Op{
				{Comment: sharedTypes.Snippet("fooBar"), Position: 0},
			},
			wantErr: false,
		},
		{
			name: "commentPassThroughComment",
			args: args{
				op: sharedTypes.Op{
					{Comment: sharedTypes.Snippet("fooBar"), Position: 0},
				},
				otherOp: sharedTypes.Op{
					{Comment: sharedTypes.Snippet("Bar"), Position: 42},
				},
			},
			want: sharedTypes.Op{
				{Comment: sharedTypes.Snippet("fooBar"), Position: 0},
			},
			wantErr: false,
		},
		{
			name: "commentExtendedByInsertion",
			args: args{
				op: sharedTypes.Op{
					{Comment: sharedTypes.Snippet("fooBaz"), Position: 0},
				},
				otherOp: sharedTypes.Op{
					{Insertion: sharedTypes.Snippet("Bar"), Position: 3},
				},
			},
			want: sharedTypes.Op{
				{Comment: sharedTypes.Snippet("fooBarBaz"), Position: 0},
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
