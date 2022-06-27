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

package project

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type Name string

func (n Name) Validate() error {
	if len(n) == 0 {
		return &errors.ValidationError{Msg: "name cannot be blank"}
	}
	if len(n) > 150 {
		return &errors.ValidationError{Msg: "name is too long"}
	}
	for _, c := range n {
		switch c {
		case '/':
			return &errors.ValidationError{Msg: "name cannot contain /"}
		case '\\':
			return &errors.ValidationError{Msg: "name cannot contain \\"}
		case '\r', '\n':
			return &errors.ValidationError{
				Msg: "name cannot contain line feeds",
			}
		}
	}
	return nil
}

var regexNumericSuffix = regexp.MustCompile("\\(\\d{1,3}\\)$")

type Names []Name

func (names Names) MakeUnique(source Name) Name {
	n := 0
	cleanName := source
	if suffix := regexNumericSuffix.FindString(string(source)); suffix != "" {
		i, err := strconv.ParseInt(suffix[1:len(suffix)-1], 10, 64)
		if err == nil {
			cleanName = Name(
				strings.TrimSpace(string(source)[:len(source)-len(suffix)]),
			)
			if cleanName == "" {
				// weird name `(1)`
				cleanName = source
			} else {
				n = int(i)
			}
		}
	}
	cleanNameIsUnique := true
	sourceNameIsUnique := true
	for _, name := range names {
		if name == cleanName {
			cleanNameIsUnique = false
		}
		if name == source {
			sourceNameIsUnique = false
		}
	}
	if cleanNameIsUnique {
		return cleanName
	}
	if sourceNameIsUnique {
		return source
	}
	for i := 1; i <= len(names); i++ {
		candidate := Name(fmt.Sprintf("%s (%d)", cleanName, n+i))
		unique := true
		for _, name := range names {
			if name == candidate {
				unique = false
				break
			}
		}
		if unique {
			return candidate
		}
	}
	// sorry, could not find unique name :/
	return cleanName
}
