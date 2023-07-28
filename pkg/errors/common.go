// Golang port of Overleaf
// Copyright (C) 2021-2023 Jakob Ackermann <das7pad@outlook.com>
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
	"time"
)

type UserFacingError interface {
	IsUserFacing()
}

type FatalError interface {
	IsFatal()
}

type Causer interface {
	Cause() error
}

func IsFatalError(err error) bool {
	_, ok := GetCause(err).(FatalError)
	return ok
}

func GetPublicMessage(err error, fallback string) string {
	if _, ok := GetCause(err).(UserFacingError); ok {
		// Include tags
		return err.Error()
	}
	return fallback
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

func (m *MergedError) Clear() {
	m.errors = m.errors[:0]
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
	m := MergedError{}
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
	_, ok := GetCause(err).(*AlreadyReportedError)
	return ok
}

type ValidationError struct {
	Msg string
}

func (v *ValidationError) Error() string {
	return v.Msg
}

func (v *ValidationError) IsUserFacing() {}

func IsValidationError(err error) bool {
	_, ok := GetCause(err).(*ValidationError)
	return ok
}

type RateLimitedError struct {
	RetryIn time.Duration
}

func (e RateLimitedError) Error() string {
	return "rate limited, try again in " + e.RetryIn.String()
}

type UnprocessableEntityError struct {
	Msg string
}

func (i *UnprocessableEntityError) Error() string {
	return "unprocessable entity: " + i.Msg
}

func (i *UnprocessableEntityError) IsUserFacing() {}

func IsUnprocessableEntityError(err error) bool {
	_, ok := GetCause(err).(*UnprocessableEntityError)
	return ok
}

type InvalidStateError struct {
	Msg string
}

func (i *InvalidStateError) IsFatal() {}

func (i *InvalidStateError) Error() string {
	return "invalid state: " + i.Msg
}

func (i *InvalidStateError) IsUserFacing() {}

func IsInvalidStateError(err error) bool {
	_, ok := GetCause(err).(*InvalidStateError)
	return ok
}

type UnauthorizedError struct {
	Reason string
}

func (e *UnauthorizedError) Error() string {
	return "unauthorized: " + e.Reason
}

func (e *UnauthorizedError) IsUserFacing() {}

func (e *UnauthorizedError) IsFatal() {}

func IsUnauthorizedError(err error) bool {
	_, ok := GetCause(err).(*UnauthorizedError)
	return ok
}

type NotAuthorizedError struct{}

func (e *NotAuthorizedError) Error() string {
	return "not authorized"
}

func (e *NotAuthorizedError) IsUserFacing() {}

func (e *NotAuthorizedError) IsFatal() {}

func IsNotAuthorizedError(err error) bool {
	_, ok := GetCause(err).(*NotAuthorizedError)
	return ok
}

type NotFoundError struct{}

func (e *NotFoundError) Error() string {
	return "not found"
}

func (e *NotFoundError) IsUserFacing() {}

func IsNotFoundError(err error) bool {
	_, ok := GetCause(err).(*NotFoundError)
	return ok
}

type CodedError struct {
	Msg string
}

func (e *CodedError) Error() string {
	return e.Msg
}

func (e *CodedError) IsUserFacing() {}

// New is a re-export of the built-in errors.New function.
var New = errors.New
