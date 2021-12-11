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

package spamSafe

import (
	"testing"

	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func TestGetSafeEmail(t *testing.T) {
	type args struct {
		email       sharedTypes.Email
		alternative string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "spam safe input",
			args: args{
				email:       "spam@safe.input",
				alternative: "alt",
			},
			want: "spam@safe.input",
		},
		{
			name: "spam input",
			args: args{
				email:       "3.14159265358979323846264338327950288@foo.bar",
				alternative: "alt",
			},
			want: "alt",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetSafeEmail(tt.args.email, tt.args.alternative); got != tt.want {
				t.Errorf("GetSafeEmail() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetSafeProjectName(t *testing.T) {
	type args struct {
		name        project.Name
		alternative string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "spam safe input",
			args: args{name: "spam safe input", alternative: "alt"},
			want: "spam safe input",
		},
		{
			name: "spam input",
			args: args{name: "https://foo.bar", alternative: "alt"},
			want: "alt",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetSafeProjectName(tt.args.name, tt.args.alternative); got != tt.want {
				t.Errorf("GetSafeProjectName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetSafeUserName(t *testing.T) {
	type args struct {
		name        string
		alternative string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "spam safe input",
			args: args{name: "spam safe input", alternative: "alt"},
			want: "spam safe input",
		},
		{
			name: "spam input",
			args: args{name: "https://foo.bar", alternative: "alt"},
			want: "alt",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetSafeUserName(tt.args.name, tt.args.alternative); got != tt.want {
				t.Errorf("GetSafeUserName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSafeEmail(t *testing.T) {
	type args struct {
		email sharedTypes.Email
	}
	//goland:noinspection SpellCheckingInspection
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "ok",
			args: args{email: "realistic-email+1@domain.sub-hyphen.com"},
			want: true,
		},
		{
			name: "glyphs 1",
			args: args{email: "safe-ëmail@domain.com"},
			want: true,
		},
		{
			name: "glyphs 2",
			args: args{email: "Բարեւ@another.domain"},
			want: true,
		},
		{
			name: "invalid domain",
			args: args{email: "notquiteRight@evil$.com"},
			want: false,
		},
		{
			name: "invalid local part",
			args: args{email: "sendME$$$@iAmAprince.com"},
			want: false,
		},
		{
			name: "too long",
			args: args{email: "3.14159265358979323846264338327950288@foo.bar"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSafeEmail(tt.args.email); got != tt.want {
				t.Errorf("IsSafeEmail() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSafeProjectName(t *testing.T) {
	type args struct {
		name project.Name
	}
	//goland:noinspection SpellCheckingInspection
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "special chars 1",
			args: args{name: "An analysis of the questions of the universe!"},
			want: true,
		},
		{
			name: "special chars 2",
			args: args{name: "Math-ematics!"},
			want: true,
		},
		{
			name: "quotes",
			args: args{name: "A'p'o's't'r'o'p'h'e gallore"},
			want: true,
		},
		{
			name: "long",
			args: args{
				name: "3.1415926535897932384626433832795028841971693993751058",
			},
			want: false,
		},
		{
			name: "too long",
			args: args{
				name: "Neural Networks: good for your health and will solve" +
					" all your problems",
			},
			want: false,
		},
		{
			name: "url",
			args: args{
				name: "come buy this =>" +
					" http://www.foo.com/search/?q=user123",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSafeProjectName(tt.args.name); got != tt.want {
				t.Errorf("IsSafeProjectName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSafeUserName(t *testing.T) {
	type args struct {
		name string
	}
	//goland:noinspection SpellCheckingInspection
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "glyphs 1",
			args: args{name: "Charline Wałęsa"},
			want: true,
		},
		{
			name: "glyphs 2",
			args: args{name: "Tammy Weinstįen"},
			want: true,
		},
		{
			name: "glyphs 3",
			args: args{name: "隆太郎 宮本"},
			want: true,
		},
		{
			name: "too long",
			args: args{
				name: "hey come buy this product im selling it's really" +
					" good for you and it'll make your latex 10x guaranteed"},
			want: false,
		},
		{
			name: "dot",
			args: args{name: "Visit haxx0red.com"},
			want: false,
		},
		{
			name: "special",
			args: args{name: "What$ Upp"},
			want: false,
		},
		{
			name: "special chars and too long",
			args: args{
				name: "加美汝VX：hihi661，金沙2001005com the first deposit" +
					" will be _100%_"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSafeUserName(tt.args.name); got != tt.want {
				t.Errorf("IsSafeUserName() = %v, want %v", got, tt.want)
			}
		})
	}
}
