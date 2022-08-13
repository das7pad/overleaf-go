// Golang port of Overleaf
// Copyright (C) 2021-2022 Jakob Ackermann <das7pad@outlook.com>
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

package outputCache

import (
	"os"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type createdDirs struct {
	base  types.CompileOutputDir
	isDir map[sharedTypes.DirName]bool
}

func (d *createdDirs) CreateBase() error {
	p := string(d.base)
	if err := os.Mkdir(p, 0o755); err != nil && !os.IsExist(err) {
		return err
	}
	d.isDir["."] = true
	return nil
}

func (d *createdDirs) EnsureIsWritable(name sharedTypes.PathName) error {
	return d.EnsureIsDir(name.Dir())
}

func (d *createdDirs) EnsureIsDir(name sharedTypes.DirName) error {
	if name == "." {
		return nil
	}
	if d.isDir[name] {
		return nil
	}
	if err := d.EnsureIsDir(name.Dir()); err != nil {
		return err
	}
	p := d.base.JoinDir(name)
	if err := os.Mkdir(p, 0o755); err != nil && !os.IsExist(err) {
		return err
	}
	d.isDir[name] = true
	return nil
}
