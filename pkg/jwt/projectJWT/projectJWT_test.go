// Golang port of Overleaf
// Copyright (C) 2024 Jakob Ackermann <das7pad@outlook.com>
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

package projectJWT

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/das7pad/overleaf-go/pkg/jwt/expiringJWT"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

var allSet = Claims{
	Claims: expiringJWT.Claims{
		ExpiresAt: 42,
	},
	AuthorizationDetails: project.AuthorizationDetails{
		Epoch:          1337,
		PrivilegeLevel: "owner",
		AccessSource:   "owner",
	},
	ProjectOptions: sharedTypes.ProjectOptions{
		CompileGroup: "priority",
		ProjectId:    sharedTypes.UUID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
		UserId:       sharedTypes.UUID{15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0},
		Timeout:      12345,
	},
	EpochUser: 21,
}

func TestClaims_tryUnmarshalJSON(t *testing.T) {
	var blob []byte
	{
		var err error
		if blob, err = json.Marshal(allSet); err != nil {
			t.Fatal(err)
		}
	}
	type args struct {
		p []byte
	}
	tests := []struct {
		name    string
		want    Claims
		args    args
		wantErr bool
	}{
		{
			name: "happy",
			args: args{p: blob},
			want: allSet,
		},
		{
			name:    "unhappy empty",
			wantErr: true,
		},
		{
			name:    "unhappy bad format",
			args:    args{p: []byte{'x'}},
			wantErr: true,
		},
		{
			name:    "unhappy unexpected field",
			args:    args{p: []byte(`{"x":1}`)},
			wantErr: true,
		},
		{
			name:    "unhappy missing value",
			args:    args{p: []byte(`{"e":}`)},
			wantErr: true,
		},
		{
			name:    "unhappy bad epoch",
			args:    args{p: []byte(`{"eu":"foo"}`)},
			wantErr: true,
		},
		{
			name:    "unhappy bad uuid",
			args:    args{p: []byte(`{"p":"x"}`)},
			wantErr: true,
		},
		{
			name:    "unhappy bad access source",
			args:    args{p: []byte(`{"s":"x"}`)},
			wantErr: true,
		},
		{
			name:    "unhappy partial field 1",
			args:    args{p: []byte(`{"e":,"eu":1}`)},
			wantErr: true,
		},
		{
			name:    "unhappy partial field 2",
			args:    args{p: []byte(`{"exp":}`)},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Claims{}
			if err := c.tryUnmarshalJSON(tt.args.p); (err != nil) != tt.wantErr {
				t.Errorf("tryUnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(c, tt.want) {
				t.Errorf("tryUnmarshalJSON() = %#v, want = %#v", c, tt.want)
			}
		})
	}
}

func Benchmark_tryUnmarshalJSON(b *testing.B) {
	blob, err := json.Marshal(allSet)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()

	c := Claims{}
	for i := 0; i < b.N; i++ {
		if err = c.tryUnmarshalJSON(blob); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkClaims_FastUnmarshalJSON(b *testing.B) {
	blob, err := json.Marshal(allSet)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()

	c := Claims{}
	for i := 0; i < b.N; i++ {
		if err = c.FastUnmarshalJSON(blob); err != nil {
			b.Fatal(err)
		}
	}
}

func Benchmark_UnmarshalJSON(b *testing.B) {
	blob, err := json.Marshal(allSet)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()

	c := Claims{}
	for i := 0; i < b.N; i++ {
		if err = json.Unmarshal(blob, &c); err != nil {
			b.Fatal(err)
		}
	}
}
