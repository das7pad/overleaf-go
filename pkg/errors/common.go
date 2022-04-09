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

package errors

import (
	"errors"
	"strings"
)

type PublicError interface {
	error
	Public() *JavaScriptError
}

type PotentiallyFatalError interface {
	IsFatal() bool
}

type Causer interface {
	Cause() error
}

func GetPublicMessage(err error) string {
	if pErr, ok := err.(PublicError); ok {
		return pErr.Error()
	}
	if cErr, ok := err.(Causer); ok {
		return GetPublicMessage(cErr.Cause())
	}
	return "internal server error"
}

type JavaScriptError struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

func (j *JavaScriptError) Error() string {
	return j.Message
}

type MergedError struct {
	errors []error
}

func (m *MergedError) Error() string {
	if len(m.errors) == 1 {
		return m.errors[0].Error()
	}
	var b strings.Builder
	b.WriteString("merged: ")
	for i, err := range m.errors {
		if i != 0 {
			b.WriteString(" + ")
		}
		b.WriteString(err.Error())
	}
	return b.String()
}

func (m *MergedError) Add(err error) {
	if err != nil {
		m.errors = append(m.errors, err)
	}
}

func (m *MergedError) Finalize() error {
	if len(m.errors) == 0 {
		return nil
	}
	if len(m.errors) == 1 {
		return m.errors[0]
	}
	return m
}

func Merge(errors ...error) error {
	m := &MergedError{}
	for _, err := range errors {
		m.Add(err)
	}
	return m.Finalize()
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

func (t *TaggedError) Unwrap() error {
	return t.cause
}

func Tag(err error, msg string) *TaggedError {
	return &TaggedError{msg: msg, cause: err}
}

func GetCause(err error) error {
	if err == nil {
		return err
	}
	if causer, ok := err.(Causer); ok {
		return GetCause(causer.Cause())
	}
	return err
}

type AlreadyReportedError struct {
	err error
}

func (a AlreadyReportedError) Error() string {
	return "already reported: " + a.err.Error()
}

func MarkAsReported(err error) error {
	return &AlreadyReportedError{err: err}
}

func IsAlreadyReported(err error) bool {
	err = GetCause(err)
	if err == nil {
		return false
	}
	_, isAlreadyReported := err.(*AlreadyReportedError)
	return isAlreadyReported
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
	err = GetCause(err)
	if err == nil {
		return false
	}
	_, isValidationError := err.(*ValidationError)
	return isValidationError
}

type UnprocessableEntityError struct {
	Msg string
}

func (i *UnprocessableEntityError) Error() string {
	return "unprocessable entity: " + i.Msg
}

func (i *UnprocessableEntityError) Public() *JavaScriptError {
	return &JavaScriptError{
		Message: i.Error(),
	}
}

func IsUnprocessableEntity(err error) bool {
	err = GetCause(err)
	if err == nil {
		return false
	}
	_, ok := err.(*UnprocessableEntityError)
	return ok
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
	err = GetCause(err)
	if err == nil {
		return false
	}
	_, ok := err.(*InvalidStateError)
	return ok
}

type UnauthorizedError struct {
	Reason string
}

func (e *UnauthorizedError) Error() string {
	return "unauthorized: " + e.Reason
}

func (e *UnauthorizedError) Public() *JavaScriptError {
	return &JavaScriptError{
		Message: e.Error(),
	}
}

func (e *UnauthorizedError) IsFatal() bool {
	return true
}

func IsUnauthorizedError(err error) bool {
	err = GetCause(err)
	if err == nil {
		return false
	}
	_, ok := err.(*UnauthorizedError)
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

func IsNotAuthorizedError(err error) bool {
	err = GetCause(err)
	if err == nil {
		return false
	}
	_, ok := err.(*NotAuthorizedError)
	return ok
}

type NotFoundError struct {
}

func (e *NotFoundError) Error() string {
	return "not found"
}

func (e *NotFoundError) Public() *JavaScriptError {
	return &JavaScriptError{
		Message: e.Error(),
	}
}

func IsNotFoundError(err error) bool {
	err = GetCause(err)
	if err == nil {
		return false
	}
	_, ok := err.(*NotFoundError)
	return ok
}

type CodedError struct {
	Description string
	Code        string
}

func (e *CodedError) Error() string {
	return e.Description + " (" + e.Code + ")"
}

func (e *CodedError) Public() *JavaScriptError {
	return &JavaScriptError{
		Message: e.Description,
		Code:    e.Code,
	}
}

// New is a re-export of the built-in errors.New function
var New = errors.New
