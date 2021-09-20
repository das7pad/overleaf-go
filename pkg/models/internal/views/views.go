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

package views

import (
	"reflect"
	"strings"
)

type View map[string]bool

func GetProjectionFor(model interface{}) View {
	projection := View{
		"_id": false,
	}
	v := reflect.TypeOf(model)
	collectFieldsFrom(v, projection)
	return projection
}

func GetFieldsOf(model interface{}) View {
	fields := View{}
	v := reflect.TypeOf(model)
	collectFieldsFrom(v, fields)
	return fields
}

func collectFieldsFrom(v reflect.Type, view View) {
	for i := 0; i < v.NumField(); i++ {
		element := v.Field(i)
		bsonTag, exists := element.Tag.Lookup("bson")
		if !exists {
			continue
		}
		if bsonTag == "inline" {
			collectFieldsFrom(element.Type, view)
		} else {
			name := strings.Split(bsonTag, ",")[0]
			view[name] = true
		}
	}
}
