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

package projectUpload

import (
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type conflictChecker map[sharedTypes.PathName]bool

func (cc conflictChecker) mkDirs(dir sharedTypes.DirName) error {
	if dir == "." {
		return nil
	}
	if err := cc.mkDirs(dir.Dir()); err != nil {
		return err
	}
	isDir, exists := cc[sharedTypes.PathName(dir)]
	if !exists {
		cc[sharedTypes.PathName(dir)] = true
		return nil
	}
	if isDir {
		return nil
	}
	return &errors.ValidationError{Msg: "conflicting paths"}
}

func (cc conflictChecker) registerFile(path sharedTypes.PathName) error {
	if err := cc.mkDirs(path.Dir()); err != nil {
		return err
	}
	if _, exits := cc[path]; exits {
		return &errors.ValidationError{Msg: "conflicting paths"}
	}
	cc[path] = false
	return nil
}
