// Golang port of Overleaf
// Copyright (C) 2022 Jakob Ackermann <das7pad@outlook.com>
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
	"reflect"
	"testing"
)

func TestUUID_String(t *testing.T) {
	tests := []struct {
		name string
		u    UUID
		want string
	}{
		{
			name: "zero",
			u:    UUID{},
			want: "00000000-0000-0000-0000-000000000000",
		},
		{
			name: "seq",
			u:    UUID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			want: "00010203-0405-0607-0809-0a0b0c0d0e0f",
		},
	}
	for _, tt := range tests {
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
				s: "00000000-0000-0000-0000-000000000000",
			},
			want:    UUID{},
			wantErr: false,
		},
		{
			name: "seq",
			args: args{
				s: "00010203-0405-0607-0809-0a0b0c0d0e0f",
			},
			want:    UUID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
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
	for _, tt := range tests {
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

func BenchmarkUUIDString(b *testing.B) {
	u := UUID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	for i := 0; i < b.N; i++ {
		_ = u.String()
	}
}

func BenchmarkUUIDParse(b *testing.B) {
	s := UUID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}.String()
	for i := 0; i < b.N; i++ {
		_, _ = ParseUUID(s)
	}
}
