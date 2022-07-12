// Golang port of Overleaf
// Copyright (C) 2022 Jakob Ackermann <das7pad@outlook.com>
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

package types

import (
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type CommonRequestOptions struct {
	CompileGroup sharedTypes.CompileGroup `json:"compileGroup"`
	ImageName    sharedTypes.ImageName    `json:"imageName"`
}

func (o *CommonRequestOptions) GetImageName() sharedTypes.ImageName {
	return o.ImageName
}

func (o *CommonRequestOptions) SetCompileGroup(cg sharedTypes.CompileGroup) {
	o.CompileGroup = cg
}

func (o *CommonRequestOptions) SetImageName(i sharedTypes.ImageName) {
	o.ImageName = i
}

func (o *CommonRequestOptions) Validate() error {
	if err := o.CompileGroup.Validate(); err != nil {
		return err
	}
	if err := o.ImageName.Validate(); err != nil {
		return err
	}
	return nil
}
