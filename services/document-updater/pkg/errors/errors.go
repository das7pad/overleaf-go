// Golang port of the Overleaf document-updater service
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

type PublicError interface {
	Public() *JavaScriptError
}

type PotentiallyFatalError interface {
	IsFatal() bool
}

type Causer interface {
	Cause() error
}

type JavaScriptError struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

func (j *JavaScriptError) Error() string {
	return j.Message
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

func GetCause(err error) error {
	if causer, ok := err.(Causer); ok {
		return GetCause(causer.Cause())
	}
	return err
}

type ValidationError struct {
	Msg string
}

func (v *ValidationError) Error() string {
	return v.Msg
}

func (v *ValidationError) Public() *JavaScriptError {
	return &JavaScriptError{
		Message: v.Error(),
	}
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

func (i *InvalidStateError) IsFatal() bool {
	return true
}

func (i *InvalidStateError) Error() string {
	return "invalid state: " + i.Msg
}

func (i *InvalidStateError) Public() *JavaScriptError {
	return &JavaScriptError{
		Message: i.Error(),
	}
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

func (e *NotAuthorizedError) Error() string {
	return "not authorized"
}

func (e *NotAuthorizedError) Public() *JavaScriptError {
	return &JavaScriptError{
		Message: e.Error(),
	}
}

func (e *NotAuthorizedError) IsFatal() bool {
	return true
}

type CodedError struct {
	Description string
	Code        string
}

func (e *CodedError) Error() string {
	return "coded error: " + e.Code
}

func (e *CodedError) Public() *JavaScriptError {
	return &JavaScriptError{
		Message: e.Description,
		Code:    e.Code,
	}
}

// New is a re-export of the built-in errors.New function
var New = errors.New
