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
	"testing"
)

func TestEmail_Host(t *testing.T) {
	tests := []struct {
		name string
		e    Email
		want string
	}{
		{
			name: "trivial",
			e:    "foo@bar.com",
			want: "bar.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.e.Host(); got != tt.want {
				t.Errorf("Host() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEmail_LocalPart(t *testing.T) {
	tests := []struct {
		name string
		e    Email
		want string
	}{
		{
			name: "trivial",
			e:    "foo@bar.com",
			want: "foo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.e.LocalPart(); got != tt.want {
				t.Errorf("LocalPart() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEmail_Normalize(t *testing.T) {
	tests := []struct {
		name string
		e    Email
		want Email
	}{
		{
			name: "trivial",
			e:    "Foo@Bar.com",
			want: "foo@bar.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.e.Normalize(); got != tt.want {
				t.Errorf("Normalize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEmail_Validate(t *testing.T) {
	tests := []struct {
		name    string
		e       Email
		wantErr bool
	}{
		{
			name:    "ok",
			e:       "foo@bar.comm",
			wantErr: false,
		},
		{
			name:    "ignore case",
			e:       "foO@baR.com",
			wantErr: false,
		},
		{
			name:    "no at",
			e:       "foo",
			wantErr: true,
		},
		{
			name:    "double at",
			e:       "@@bar.com",
			wantErr: true,
		},
		{
			name:    "space",
			e:       "foo @bar.com",
			wantErr: true,
		},
		{
			name:    "name and email",
			e:       "name <foo@bar.com>",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.e.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEmail_ReversedHostname(t *testing.T) {
	tests := []struct {
		name string
		e    Email
		want ReversedHostname
	}{
		{
			name: "trivial uneven length",
			e:    "foo@bar.com",
			want: "moc.rab",
		},
		{
			name: "trivial even length",
			e:    "foo@bar.de",
			want: "ed.rab",
		},
		{
			name: "unicode",
			e:    "foo@bär.com",
			want: "moc.räb",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.e.ReversedHostname(); got != tt.want {
				t.Errorf("ReversedHostname() = %v, want %v", got, tt.want)
			}
		})
	}
}
