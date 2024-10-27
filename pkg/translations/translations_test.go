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

package translations

import (
	"fmt"
	"html/template"
	"testing"
)

func Test_parseLocales(t *testing.T) {
	type fakeSettings struct {
		AppName string
	}
	type renderData struct {
		Settings  fakeSettings
		PoweredBy string
	}
	data := renderData{
		PoweredBy: "Golang",
		Settings: fakeSettings{
			AppName: "OverleafFromData",
		},
	}
	type args struct {
		raw     map[string]string
		appName string
	}
	type tCase struct {
		key  string
		data interface{}
		want template.HTML
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		tCases  []tCase
	}{
		{
			name: "works",
			args: args{
				appName: "OverleafFromBoot",
				raw: map[string]string{
					"no_var":           "Hello World!",
					"app_name":         "Golang port of {{ .Settings.AppName }}",
					"var":              "Powered by: {{ .PoweredBy }}",
					"var_and_app_name": "{{ .PoweredBy }} + {{ .Settings.AppName }}",
				},
			},
			tCases: []tCase{
				{
					key:  "no_var",
					data: nil,
					want: "Hello World!",
				},
				{
					key:  "app_name",
					data: nil,
					want: "Golang port of OverleafFromBoot",
				},
				{
					key:  "var",
					data: data,
					want: "Powered by: Golang",
				},
				{
					key:  "var_and_app_name",
					data: data,
					want: "Golang + OverleafFromData",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLocales(tt.args.raw, tt.args.appName)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseLocales() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			for _, ttt := range tt.tCases {
				t.Run(fmt.Sprintf("%s/%s", tt.name, ttt.key), func(t *testing.T) {
					v, err1 := got[ttt.key].Render(ttt.data)
					if err1 != nil {
						t.Errorf(
							"parseLocales()[%s].Render(%v) error = %v",
							ttt.key, ttt.data, err1,
						)
						return
					}
					if v != ttt.want {
						t.Errorf(
							"parseLocales()[%s].Render(%v) got = %q, want %q",
							ttt.key, ttt.data, v, ttt.want,
						)
						return
					}
				})
			}
		})
	}
}
