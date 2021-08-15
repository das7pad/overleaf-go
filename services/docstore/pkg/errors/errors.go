// Golang port of the Overleaf docstore service
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

type ValidationError struct {
	Msg string
}

func (c ValidationError) Error() string {
	return c.Msg
}

type BodyTooLargeError struct {
	ValidationError
}

func (e BodyTooLargeError) Error() string {
	return "document body too large"
}

func IsBodyTooLargeError(err error) bool {
	if err == nil {
		return false
	}
	_, isBodyTooLargeError := err.(BodyTooLargeError)
	return isBodyTooLargeError
}

func IsValidationError(err error) bool {
	if err == nil {
		return false
	}
	_, isValidationError := err.(ValidationError)
	return isValidationError
}

func IsDocNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	_, isNotFoundErr := err.(ErrorDocNotFound)
	return isNotFoundErr
}

type ErrorDocNotFound struct {
}

func (e ErrorDocNotFound) Error() string {
	return "doc not found"
}

type ErrorDocArchived struct {
}

func (e ErrorDocArchived) Error() string {
	return "doc is archived"
}

func IsDocArchivedError(err error) bool {
	if err == nil {
		return false
	}
	_, isArchivedErr := err.(ErrorDocArchived)
	return isArchivedErr
}
