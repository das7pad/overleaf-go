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

package types

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/document-updater/pkg/errors"
)

type coreProjectUpdate struct {
	Id      primitive.ObjectID `json:"id"`
	Version string             `json:"version"`
	Type    string             `json:"type"`
}

type RenameUpdate struct {
	coreProjectUpdate
	PathName    PathName `json:"pathname"`
	NewPathName PathName `json:"newPathname"`
}

func (r *RenameUpdate) Validate() error {
	if r.PathName == "" {
		return &errors.ValidationError{Msg: "missing old path"}
	}
	return nil
}

type GenericProjectUpdate struct {
	coreProjectUpdate
	PathName    PathName `json:"pathname"`
	NewPathName PathName `json:"newPathname"`
}

func (g *GenericProjectUpdate) RenameUpdate() *RenameUpdate {
	return &RenameUpdate{
		coreProjectUpdate: g.coreProjectUpdate,
		PathName:          g.PathName,
		NewPathName:       g.NewPathName,
	}
}

type ProcessProjectUpdatesRequest struct {
	ProjectVersion Version                `json:"version"`
	Updates        []GenericProjectUpdate `json:"updates"`
}

func (p *ProcessProjectUpdatesRequest) Validate() error {
	if p.ProjectVersion < 0 {
		return &errors.ValidationError{Msg: "version must be greater 0"}
	}
	if len(p.Updates) == 0 {
		return &errors.ValidationError{Msg: "missing updates"}
	}
	for _, update := range p.Updates {
		switch update.Type {
		case "rename-doc":
			if err := update.RenameUpdate().Validate(); err != nil {
				return err
			}
		case "rename-file":
		case "add-doc":
		case "add-file":
		default:
			return &errors.ValidationError{
				Msg: "unknown update type: " + update.Type,
			}
		}
	}
	return nil
}
