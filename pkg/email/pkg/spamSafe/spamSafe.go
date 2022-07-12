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

package spamSafe

import (
	"regexp"

	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

var (
	emailRegex = regexp.MustCompile(`^[\p{L}\p{N}.+_-]+@[\w.-]+$`)
	hanRegex   = regexp.MustCompile(`\p{Han}`)
	safeRegex  = regexp.MustCompile(`^[\p{L}\p{N}\s\-_!'&()]+$`)
)

func GetSafeEmail(email sharedTypes.Email, alternative string) string {
	if IsSafeEmail(email) {
		return string(email)
	}
	return alternative
}

func GetSafeProjectName(name project.Name, alternative string) string {
	if IsSafeProjectName(name) {
		return string(name)
	}
	return alternative
}

func GetSafeUserName(name, alternative string) string {
	if IsSafeUserName(name) {
		return name
	}
	return alternative
}

func IsSafeEmail(email sharedTypes.Email) bool {
	return len(email) <= 40 && emailRegex.MatchString(string(email))
}

func IsSafeProjectName(name project.Name) bool {
	if len(name) > 100 {
		return false
	}
	s := string(name)
	if hanRegex.MatchString(s) {
		return len(name) <= 30
	}
	return safeRegex.MatchString(s)
}

func IsSafeUserName(name string) bool {
	return len(name) <= 30 && safeRegex.MatchString(name)
}
