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
	data := struct {
		Settings struct {
			AppName string
		}
		Lng string
	}{
		Lng: "English",
		Settings: struct {
			AppName string
		}{
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
					"var":              "Translated into {{ .Lng }}",
					"var_and_app_name": "{{ .Settings.AppName }} in {{ .Lng }}",
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
					want: "Translated into English",
				},
				{
					key:  "var_and_app_name",
					data: data,
					want: "OverleafFromData in English",
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
