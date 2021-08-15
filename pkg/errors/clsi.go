// Golang port Overleaf
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
