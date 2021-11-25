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

package types

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type coreProjectUpdate struct {
	Id       primitive.ObjectID   `json:"id"`
	PathName sharedTypes.PathName `json:"pathname"`
	Version  string               `json:"version"`
	Type     string               `json:"type"`
}

type AddDocUpdate struct {
	coreProjectUpdate
}

func (a *AddDocUpdate) Validate() error {
	if a.PathName == "" {
		return &errors.ValidationError{Msg: "missing path"}
	}
	return nil
}

func (a *AddDocUpdate) ToGeneric() *GenericProjectUpdate {
	return &GenericProjectUpdate{
		coreProjectUpdate: a.coreProjectUpdate,
	}
}

type AddFileUpdate struct {
	coreProjectUpdate
	URL string `json:"url"`
}

func (a *AddFileUpdate) Validate() error {
	if a.PathName == "" {
		return &errors.ValidationError{Msg: "missing path"}
	}
	if a.URL == "" {
		return &errors.ValidationError{Msg: "missing url"}
	}
	return nil
}

type RenameDocUpdate struct {
	coreProjectUpdate
	NewPathName sharedTypes.PathName `json:"newPathname"`
}

func (r *RenameDocUpdate) Validate() error {
	if r.PathName == "" {
		return &errors.ValidationError{Msg: "missing old path"}
	}
	return nil
}

func (r *RenameDocUpdate) ToGeneric() *GenericProjectUpdate {
	return &GenericProjectUpdate{
		coreProjectUpdate: r.coreProjectUpdate,
		NewPathName:       r.NewPathName,
	}
}

func NewRenameDocUpdate(id primitive.ObjectID, oldPath, newPath sharedTypes.PathName) *RenameDocUpdate {
	return &RenameDocUpdate{
		coreProjectUpdate: coreProjectUpdate{
			Id:       id,
			PathName: oldPath,
			Type:     "rename-doc",
		},
		NewPathName: newPath,
	}
}

type RenameFileUpdate struct {
	coreProjectUpdate
	NewPathName sharedTypes.PathName `json:"newPathname"`
}

func (r *RenameFileUpdate) Validate() error {
	if r.PathName == "" {
		return &errors.ValidationError{Msg: "missing old path"}
	}
	return nil
}

func (r *RenameFileUpdate) ToGeneric() *GenericProjectUpdate {
	return &GenericProjectUpdate{
		coreProjectUpdate: r.coreProjectUpdate,
		NewPathName:       r.NewPathName,
	}
}

type GenericProjectUpdate struct {
	coreProjectUpdate
	NewPathName sharedTypes.PathName `json:"newPathname"`
	URL         string               `json:"url"`
}

func (g *GenericProjectUpdate) AddDocUpdate() *AddDocUpdate {
	return &AddDocUpdate{
		coreProjectUpdate: g.coreProjectUpdate,
	}
}

func (g *GenericProjectUpdate) AddFileUpdate() *AddFileUpdate {
	return &AddFileUpdate{
		coreProjectUpdate: g.coreProjectUpdate,
		URL:               g.URL,
	}
}

func (g *GenericProjectUpdate) RenameDocUpdate() *RenameDocUpdate {
	return &RenameDocUpdate{
		coreProjectUpdate: g.coreProjectUpdate,
		NewPathName:       g.NewPathName,
	}
}

func (g *GenericProjectUpdate) RenameFileUpdate() *RenameFileUpdate {
	return &RenameFileUpdate{
		coreProjectUpdate: g.coreProjectUpdate,
		NewPathName:       g.NewPathName,
	}
}

type ProcessProjectUpdatesRequest struct {
	ProjectVersion sharedTypes.Version     `json:"version"`
	Updates        []*GenericProjectUpdate `json:"updates"`
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
		case "add-doc":
			if err := update.AddDocUpdate().Validate(); err != nil {
				return err
			}
		case "add-file":
			if err := update.AddFileUpdate().Validate(); err != nil {
				return err
			}
		case "rename-doc":
			if err := update.RenameDocUpdate().Validate(); err != nil {
				return err
			}
		case "rename-file":
			if err := update.RenameFileUpdate().Validate(); err != nil {
				return err
			}
		default:
			return &errors.ValidationError{
				Msg: "unknown update type: " + update.Type,
			}
		}
	}
	return nil
}
