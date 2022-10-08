// Golang port of Overleaf
// Copyright (C) 2021-2022 Jakob Ackermann <das7pad@outlook.com>
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
	"net/url"
	"reflect"
	"testing"
)

func TestURL_MarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		u       URL
		want    []byte
		wantErr bool
	}{
		{
			name: "happy path",
			u: URL{url.URL{
				Scheme: "https",
				Host:   "example.com",
				Path:   "/file",
			}},
			want:    []byte(`"https://example.com/file"`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.u.MarshalJSON()
			if (err != nil) != tt.wantErr {
				t.Errorf("MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MarshalJSON() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestURL_UnmarshalJSON(t *testing.T) {
	type args struct {
		bytes []byte
	}
	tests := []struct {
		name    string
		u       URL
		args    args
		want    URL
		wantErr bool
	}{
		{
			name: "happy path",
			args: args{
				bytes: []byte(`"https://example.com/file"`),
			},
			want: URL{url.URL{
				Scheme: "https",
				Host:   "example.com",
				Path:   "/file",
			}},
			wantErr: false,
		},
		{
			name: "missing schema",
			args: args{
				bytes: []byte(`"//example.com/file"`),
			},
			wantErr: true,
		},
		{
			name: "missing host",
			args: args{
				bytes: []byte(`"example.com/file"`),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.u.UnmarshalJSON(tt.args.bytes); (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(tt.u, tt.want) {
				t.Errorf("UnmarshalJSON() got = %v, want %v", tt.u, tt.want)
			}
		})
	}
}

func TestURL_Validate(t *testing.T) {
	tests := []struct {
		name    string
		u       URL
		wantErr bool
	}{
		{
			name: "happy path",
			u: URL{url.URL{
				Scheme: "https",
				Host:   "example.com",
				Path:   "/file",
			}},
			wantErr: false,
		},
		{
			name: "missing schema",
			u: URL{url.URL{
				Host: "example.com",
				Path: "/file",
			}},
			wantErr: true,
		},
		{
			name: "missing host",
			u: URL{url.URL{
				Path: "example.com/file",
			}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.u.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestURL_WithPath(t *testing.T) {
	buildBase := func() url.URL {
		u, _ := url.Parse("http://foo.bar/baz?bazz=barr")
		return *u
	}

	t.Run("value base", func(t *testing.T) {
		u := URL{URL: buildBase()}
		x := u.WithPath("/other")
		if got := x.Path; got != "/other" {
			t.Errorf("WithPath().Path = %v, want %v", got, "/other")
		}
		if got := u.Path; got != "/baz" {
			t.Errorf("u.WithPath(); u.Path = %v, want %v", got, "/baz")
		}
	})

	t.Run("pointer base", func(t *testing.T) {
		u := &URL{URL: buildBase()}
		x := u.WithPath("/other")
		if got := x.Path; got != "/other" {
			t.Errorf("WithPath().Path = %v, want %v", got, "/other")
		}
		if got := u.Path; got != "/baz" {
			t.Errorf("u.WithPath(); u.Path = %v, want %v", got, "/baz")
		}
	})
}

func TestURL_WithQuery(t *testing.T) {
	buildBase := func() url.URL {
		u, _ := url.Parse("http://foo.bar/baz?key=old")
		return *u
	}

	t.Run("value base", func(t *testing.T) {
		u := URL{URL: buildBase()}
		x := u.WithQuery(url.Values{"key": {"new"}})
		if got := x.Query().Get("key"); got != "new" {
			t.Errorf("Query().Query().Get() = %v, want %v", got, "new")
		}
		if got := u.Query().Get("key"); got != "old" {
			t.Errorf("u.Query(); u.Query().Get() = %v, want %v", got, "old")
		}
	})

	t.Run("pointer base", func(t *testing.T) {
		u := &URL{URL: buildBase()}
		x := u.WithPath("/other")
		if got := x.Path; got != "/other" {
			t.Errorf("WithPath().Path = %v, want %v", got, "/other")
		}
		if got := u.Path; got != "/baz" {
			t.Errorf("u.WithPath(); u.Path = %v, want %v", got, "/baz")
		}
	})
}
