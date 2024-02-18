// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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
	"testing"
)

func TestPathName_Dir(t *testing.T) {
	tests := []struct {
		name string
		f    PathName
		want DirName
	}{
		{
			"no dir",
			"foo.txt",
			".",
		},
		{
			"dir",
			"foo/bar.txt",
			"foo",
		},
		{
			"dirs",
			"foo/bar/baz.txt",
			"foo/bar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.f.Dir(); got != tt.want {
				t.Errorf("Dir() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPathName_Type(t *testing.T) {
	tests := []struct {
		name string
		f    PathName
		want FileType
	}{
		{
			"exists",
			"foo.txt",
			"txt",
		},
		{
			"missing",
			"foo",
			"",
		},
		{
			"multi",
			"foo.txt.tex",
			"tex",
		},
		{
			"ends with dot",
			"foo.",
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.f.Type(); got != tt.want {
				t.Errorf("Type() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPathName_Validate(t *testing.T) {
	tests := []struct {
		name    string
		f       PathName
		wantErr bool
	}{
		{
			"ok",
			"foo.txt",
			false,
		},
		{
			"zero size",
			"",
			true,
		},
		{
			"absolute",
			"/foo.txt",
			true,
		},
		{
			"dir explicit",
			"foo/",
			true,
		},
		{
			"dir implicit",
			"foo/.",
			true,
		},
		{
			"just dot",
			".",
			true,
		},
		{
			"just dots",
			"..",
			true,
		},
		{
			"jumping parent",
			"..",
			true,
		},
		{
			"jumping start",
			"../foo.txt",
			true,
		},
		{
			"jumping middle",
			"foo/../bar.txt",
			true,
		},
		{
			"jumping end",
			"foo/..",
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.f.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPathName_Filename(t *testing.T) {
	tests := []struct {
		name string
		p    PathName
		want Filename
	}{
		{
			"trivial",
			"foo/bar.txt",
			"bar.txt",
		},
		{
			"multiple",
			"foo/bar/baz.txt",
			"baz.txt",
		},
		{
			"no slash",
			"bar.txt",
			"bar.txt",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.Filename(); got != tt.want {
				t.Errorf("Filename() = %v, want %v", got, tt.want)
			}
		})
	}
}
