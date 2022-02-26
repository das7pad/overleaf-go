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

package types

import (
	"reflect"
	"testing"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func TestDoc_DeleteReviewThread(t *testing.T) {
	idA, _ := edgedb.ParseUUID("012345678901234567890123")
	idB, _ := edgedb.ParseUUID("987654321098765432109876")
	type fields struct {
		DocCore             DocCore
		LastUpdatedCtx      LastUpdatedCtx
		Version             sharedTypes.Version
		UnFlushedTime       UnFlushedTime
		DocId               edgedb.UUID
		JustLoadedIntoRedis bool
	}
	type args struct {
		threadId edgedb.UUID
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
		after  sharedTypes.Ranges
	}{
		{
			name: "empty",
			fields: fields{
				DocCore: DocCore{
					Ranges: sharedTypes.Ranges{},
				},
			},
			args:  args{threadId: idA},
			want:  false,
			after: sharedTypes.Ranges{},
		},
		{
			name: "no match",
			fields: fields{
				DocCore: DocCore{
					Ranges: sharedTypes.Ranges{
						Comments: []sharedTypes.Comment{
							{
								RangeEntryBase: sharedTypes.RangeEntryBase{
									Id: idB,
								},
							},
						},
					},
				},
			},
			args: args{threadId: idA},
			want: false,
			after: sharedTypes.Ranges{
				Comments: []sharedTypes.Comment{
					{
						RangeEntryBase: sharedTypes.RangeEntryBase{
							Id: idB,
						},
					},
				},
			},
		},
		{
			name: "match last",
			fields: fields{
				DocCore: DocCore{
					Ranges: sharedTypes.Ranges{
						Comments: []sharedTypes.Comment{
							{
								RangeEntryBase: sharedTypes.RangeEntryBase{
									Id: idA,
								},
							},
						},
					},
				},
			},
			args: args{threadId: idA},
			want: true,
			after: sharedTypes.Ranges{
				Comments: []sharedTypes.Comment{},
			},
		},
		{
			name: "match first",
			fields: fields{
				DocCore: DocCore{
					Ranges: sharedTypes.Ranges{
						Comments: []sharedTypes.Comment{
							{
								RangeEntryBase: sharedTypes.RangeEntryBase{
									Id: idA,
								},
							},
							{
								RangeEntryBase: sharedTypes.RangeEntryBase{
									Id: idB,
								},
							},
						},
					},
				},
			},
			args: args{threadId: idA},
			want: true,
			after: sharedTypes.Ranges{
				Comments: []sharedTypes.Comment{
					{
						RangeEntryBase: sharedTypes.RangeEntryBase{
							Id: idB,
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Doc{
				DocCore:             tt.fields.DocCore,
				LastUpdatedCtx:      tt.fields.LastUpdatedCtx,
				Version:             tt.fields.Version,
				UnFlushedTime:       tt.fields.UnFlushedTime,
				DocId:               tt.fields.DocId,
				JustLoadedIntoRedis: tt.fields.JustLoadedIntoRedis,
			}
			got := d.DeleteReviewThread(tt.args.threadId)
			if got != tt.want {
				t.Errorf("DeleteReviewThread() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(d.Ranges, tt.after) {
				t.Errorf("d.DeleteReviewThread(); d.Ranges = %v, want %v", d.Ranges, tt.after)
			}
		})
	}
}
