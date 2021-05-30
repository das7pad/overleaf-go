// Golang port of the Overleaf clsi service
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
	"testing"
)

func TestFileName_Dir(t *testing.T) {
	tests := []struct {
		name string
		f    FileName
		want FileName
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

func TestFileName_Type(t *testing.T) {
	tests := []struct {
		name string
		f    FileName
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

func TestFileName_Validate(t *testing.T) {
	type args struct {
		in0 *Options
	}
	noArgs := args{}
	tests := []struct {
		name    string
		f       FileName
		args    args
		wantErr bool
	}{
		{
			"ok",
			"foo.txt",
			noArgs,
			false,
		},
		{
			"zero size",
			"",
			noArgs,
			true,
		},
		{
			"absolute",
			"/foo.txt",
			noArgs,
			true,
		},
		{
			"dir explicit",
			"foo/",
			noArgs,
			true,
		},
		{
			"dir implicit",
			"foo/.",
			noArgs,
			true,
		},
		{
			"just dot",
			".",
			noArgs,
			true,
		},
		{
			"just dots",
			"..",
			noArgs,
			true,
		},
		{
			"jumping parent",
			"..",
			noArgs,
			true,
		},
		{
			"jumping start",
			"../foo.txt",
			noArgs,
			true,
		},
		{
			"jumping middle",
			"foo/../bar.txt",
			noArgs,
			true,
		},
		{
			"jumping end",
			"foo/..",
			noArgs,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.f.Validate(tt.args.in0); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
