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

type Causer interface {
	Cause() error
}

type TaggedError struct {
	msg   string
	cause error
}

func (t *TaggedError) Error() string {
	return t.msg + ": " + t.cause.Error()
}

func (t *TaggedError) Cause() error {
	return t.cause
}

func Tag(err error, msg string) *TaggedError {
	return &TaggedError{msg: msg, cause: err}
}

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

type InvalidStateError struct {
	Msg string
}

func (i InvalidStateError) Error() string {
	return "invalid state: " + i.Msg
}

func IsInvalidState(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*InvalidStateError)
	return ok
}

type NotAuthorizedError struct {
}

func (e NotAuthorizedError) Error() string {
	return "not authorized"
}

// New is a re-export of the built-in errors.New function
var New = errors.New
