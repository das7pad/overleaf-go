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
	Id      primitive.ObjectID `json:"id"`
	Version string             `json:"version"`
	Type    string             `json:"type"`
}

type AddDocUpdate struct {
	coreProjectUpdate
	PathName sharedTypes.PathName `json:"pathname"`
}

func (a *AddDocUpdate) Validate() error {
	if a.PathName == "" {
		return &errors.ValidationError{Msg: "missing path"}
	}
	return nil
}

type AddFileUpdate struct {
	coreProjectUpdate
	PathName sharedTypes.PathName `json:"pathname"`
	URL      string               `json:"url"`
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
	PathName    sharedTypes.PathName `json:"pathname"`
	NewPathName sharedTypes.PathName `json:"newPathname"`
}

func (r *RenameDocUpdate) Validate() error {
	if r.PathName == "" {
		return &errors.ValidationError{Msg: "missing old path"}
	}
	return nil
}

type RenameFileUpdate struct {
	coreProjectUpdate
	PathName    sharedTypes.PathName `json:"pathname"`
	NewPathName sharedTypes.PathName `json:"newPathname"`
	URL         string               `json:"url"`
}

func (r *RenameFileUpdate) Validate() error {
	if r.PathName == "" {
		return &errors.ValidationError{Msg: "missing old path"}
	}
	if r.URL == "" {
		return &errors.ValidationError{Msg: "missing url"}
	}
	return nil
}

type GenericProjectUpdate struct {
	coreProjectUpdate
	PathName    sharedTypes.PathName `json:"pathname"`
	NewPathName sharedTypes.PathName `json:"newPathname"`
	URL         string               `json:"url"`
}

func (g *GenericProjectUpdate) AddDocUpdate() *AddDocUpdate {
	return &AddDocUpdate{
		coreProjectUpdate: g.coreProjectUpdate,
		PathName:          g.PathName,
	}
}

func (g *GenericProjectUpdate) AddFileUpdate() *AddFileUpdate {
	return &AddFileUpdate{
		coreProjectUpdate: g.coreProjectUpdate,
		PathName:          g.PathName,
		URL:               g.URL,
	}
}

func (g *GenericProjectUpdate) RenameDocUpdate() *RenameDocUpdate {
	return &RenameDocUpdate{
		coreProjectUpdate: g.coreProjectUpdate,
		PathName:          g.PathName,
		NewPathName:       g.NewPathName,
	}
}

func (g *GenericProjectUpdate) RenameFileUpdate() *RenameFileUpdate {
	return &RenameFileUpdate{
		coreProjectUpdate: g.coreProjectUpdate,
		PathName:          g.PathName,
		NewPathName:       g.NewPathName,
		URL:               g.URL,
	}
}

type ProcessProjectUpdatesRequest struct {
	ProjectVersion sharedTypes.Version    `json:"version"`
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
