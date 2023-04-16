// Golang port of Overleaf
// Copyright (C) 2021-2023 Jakob Ackermann <das7pad@outlook.com>
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

package proxyClient

import (
	"net/url"
	"testing"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func Test_chainURL(t *testing.T) {
	mustParse := func(s string) *sharedTypes.URL {
		u, err := sharedTypes.ParseAndValidateURL(s)
		if err != nil {
			t.Fatalf("cannot parse url: %q: %q", s, err.Error())
		}
		return u
	}
	type args struct {
		u     *sharedTypes.URL
		chain []sharedTypes.URL
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "works 1 stage: evil query",
			args: args{
				u: mustParse("https://foo.bar.com/p1/p2?url=evil"),
				chain: []sharedTypes.URL{
					*mustParse("https://first.com/x1/x2"),
				},
			},
			want: "https://first.com/x1/x2?url=" + url.QueryEscape(
				"https://foo.bar.com/p1/p2?url=evil",
			),
		},
		{
			name: "works 1 stage: second evil url param",
			args: args{
				u: mustParse("https://foo.bar.com/p1/p2&url=evil"),
				chain: []sharedTypes.URL{
					*mustParse("https://first.com/x1/x2"),
				},
			},
			want: "https://first.com/x1/x2?url=" + url.QueryEscape(
				"https://foo.bar.com/p1/p2&url=evil",
			),
		},
		{
			name: "works 3 stages",
			args: args{
				u: mustParse("https://foo.bar.com/p1/p2&url=evil"),
				chain: []sharedTypes.URL{
					*mustParse("https://first.com/x1/x2"),
					*mustParse("https://second.com/y1/y2"),
					*mustParse("https://third.com/z1/z2"),
				},
			},
			want: "https://third.com/z1/z2?next_is_proxy=true&url=" +
				url.QueryEscape(
					"https://second.com/y1/y2?next_is_proxy=true&url="+
						url.QueryEscape(
							"https://first.com/x1/x2?url="+
								url.QueryEscape(
									"https://foo.bar.com/p1/p2&url=evil",
								),
						),
				),
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			if got := chainURL(tt.args.u, tt.args.chain); got != tt.want {
				t.Errorf("chainURL() = %v, want %v", got, tt.want)
				return
			}
		})
		t.Run(tt.name+" round trip", func(t *testing.T) {
			v := mustParse(tt.want)
			for i := 0; i < len(tt.args.chain); i++ {
				if v.Query().Get("url") == "evil" {
					t.Errorf("broken chaining, got evil url")
					return
				}
				v = mustParse(v.Query().Get("url"))
			}
			if v.String() != tt.args.u.String() {
				t.Errorf(
					"cannot round trip url: got %v, want %v",
					v.String(), tt.args.u.String(),
				)
				return
			}
		})
	}
}
