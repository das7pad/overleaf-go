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

package sharedTypes

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func TestUUID_IsZero(t *testing.T) {
	tests := []struct {
		name string
		u    UUID
		want bool
	}{
		{
			name: "zero",
			u:    UUID{},
			want: true,
		},
		{
			name: "set",
			u:    UUID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 255},
			want: false,
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.u.IsZero(); got != tt.want {
				t.Errorf("IsZero() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUUID_String(t *testing.T) {
	tests := []struct {
		name string
		u    UUID
		want string
	}{
		{
			name: "zero",
			u:    UUID{},
			want: AllZeroUUID,
		},
		{
			name: "seq",
			u:    UUID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 255},
			want: "00010203-0405-0607-0809-0a0b0c0d0eff",
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.u.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseUUID(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name    string
		args    args
		want    UUID
		wantErr bool
	}{
		{
			name: "zero",
			args: args{
				s: AllZeroUUID,
			},
			want:    UUID{},
			wantErr: false,
		},
		{
			name: "seq",
			args: args{
				s: "00010203-0405-0607-0809-0a0b0c0d0eff",
			},
			want:    UUID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 255},
			wantErr: false,
		},
		{
			name: "short",
			args: args{
				s: "0",
			},
			want:    UUID{},
			wantErr: true,
		},
		{
			name: "bad format",
			args: args{
				s: "000000000000000000000000000000000000",
			},
			want:    UUID{},
			wantErr: true,
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseUUID(tt.args.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseUUID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseUUID() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkUUID_IsZero(b *testing.B) {
	b.ReportAllocs()
	u := UUID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 255}
	for i := 0; i < b.N; i++ {
		_ = u.IsZero()
	}
}

func BenchmarkUUID_String(b *testing.B) {
	b.ReportAllocs()
	u := UUID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 255}
	for i := 0; i < b.N; i++ {
		_ = u.String()
	}
}

func BenchmarkUUID_StringNaive(b *testing.B) {
	b.ReportAllocs()
	u := UUID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 255}
	for i := 0; i < b.N; i++ {
		_ = fmt.Sprintf(
			"%x-%x-%x-%x-%x",
			u[0:4], u[4:6], u[6:8], u[8:10], u[10:16],
		)
	}
}

func BenchmarkUUID_MarshalJSON(b *testing.B) {
	b.ReportAllocs()
	u := UUID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 255}
	for i := 0; i < b.N; i++ {
		_, _ = u.MarshalJSON()
	}
}

func BenchmarkUUID_MarshalJSONNaive(b *testing.B) {
	b.ReportAllocs()
	u := UUID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 255}
	for i := 0; i < b.N; i++ {
		if _, err := json.Marshal(u.String()); err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkUUID_UnmarshalJSON(b *testing.B) {
	b.ReportAllocs()
	u := UUID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 255}
	blob, err := json.Marshal(u)
	if err != nil {
		b.Error(err)
	}
	u2 := &UUID{}
	for i := 0; i < b.N; i++ {
		err = u2.UnmarshalJSON(blob)
		if err != nil {
			b.Error(err)
		}
		if u != *u2 {
			b.Fatalf("%s != %s", u, u2)
		}
	}
}

func BenchmarkUUID_UnmarshalJSONNaive(b *testing.B) {
	b.ReportAllocs()
	u := UUID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 255}
	blob, err := json.Marshal(u)
	if err != nil {
		b.Error(err)
	}
	u2 := &UUID{}
	for i := 0; i < b.N; i++ {
		s := ""
		err = json.Unmarshal(blob, &s)
		if err != nil {
			b.Error(err)
		}
		var u3 UUID
		u3, err = ParseUUID(s)
		if err != nil {
			b.Error(err)
		}
		*u2 = u3
		if u != *u2 {
			b.Fatalf("%s != %s", u, u2)
		}
	}
}

func BenchmarkParseUUID(b *testing.B) {
	b.ReportAllocs()
	s := UUID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 255}.String()
	for i := 0; i < b.N; i++ {
		_, _ = ParseUUID(s)
	}
}

func BenchmarkPopulateUUID(b *testing.B) {
	b.ReportAllocs()
	u := UUID{}
	for i := 0; i < b.N; i++ {
		_ = PopulateUUID(&u)
	}
}
