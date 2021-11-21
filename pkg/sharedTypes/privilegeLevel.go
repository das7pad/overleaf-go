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

package sharedTypes

import (
	"github.com/das7pad/overleaf-go/pkg/errors"
)

type PrivilegeLevel string

const (
	PrivilegeLevelOwner        PrivilegeLevel = "owner"
	PrivilegeLevelReadAndWrite PrivilegeLevel = "readAndWrite"
	PrivilegeLevelReadOnly     PrivilegeLevel = "readOnly"
)

func (l PrivilegeLevel) score() int {
	switch l {
	case PrivilegeLevelOwner:
		return 3
	case PrivilegeLevelReadAndWrite:
		return 2
	case PrivilegeLevelReadOnly:
		return 1
	default:
		return 0
	}
}

func (l PrivilegeLevel) CheckIsAtLeast(other PrivilegeLevel) error {
	if !l.IsAtLeast(other) {
		return &errors.NotAuthorizedError{}
	}
	return nil
}

func (l PrivilegeLevel) IsAtLeast(other PrivilegeLevel) bool {
	return l.score() >= other.score()
}

func (l PrivilegeLevel) IsHigherThan(other PrivilegeLevel) bool {
	return l.score() > other.score()
}

func (l PrivilegeLevel) Validate() error {
	if l.score() == 0 {
		return &errors.ValidationError{Msg: "invalid PrivilegeLevel"}
	}
	return nil
}
