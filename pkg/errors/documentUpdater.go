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

type UpdateRangeNotAvailableError struct {
}

func (i *UpdateRangeNotAvailableError) IsFatal() bool {
	return true
}

func (i *UpdateRangeNotAvailableError) Error() string {
	return "doc ops range is not loaded in redis"
}

func (i *UpdateRangeNotAvailableError) Public() *JavaScriptError {
	return &JavaScriptError{
		Message: i.Error(),
	}
}

func IsUpdateRangeNotAvailableError(err error) bool {
	err = GetCause(err)
	if err == nil {
		return false
	}
	_, ok := err.(*UpdateRangeNotAvailableError)
	return ok
}
