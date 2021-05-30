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

package errors

import (
	"errors"
)

type ValidationError struct {
	Msg string
}

func (c ValidationError) Error() string {
	return c.Msg
}

func IsValidationError(err error) bool {
	if err == nil {
		return false
	}
	_, isValidationError := err.(*ValidationError)
	return isValidationError
}

type MissingOutputFileError struct {
	Msg string
}

func (m MissingOutputFileError) Error() string {
	return m.Msg
}

func IsMissingOutputFileError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*MissingOutputFileError)
	return ok
}

type InvalidStateError struct {
	Msg         string
	Recoverable bool
}

func (i InvalidStateError) Error() string {
	return "invalid state: " + i.Msg
}

func IsRecoverable(err error) bool {
	if err == nil {
		return false
	}
	invalidStateErr, ok := err.(*InvalidStateError)
	if !ok {
		return false
	}
	return invalidStateErr.Recoverable
}

func IsInvalidState(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*InvalidStateError)
	return ok
}

type AlreadyCompilingError struct {
}

func (a AlreadyCompilingError) Error() string {
	return "already compiling"
}

func IsAlreadyCompiling(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*AlreadyCompilingError)
	return ok
}

var (
	ProjectHasTooManyFilesAndDirectories = InvalidStateError{
		Msg: "project has too many files/directories",
	}
)

// New is a re-export of the built-in errors.New function
var New = errors.New
