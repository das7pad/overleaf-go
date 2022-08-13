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

package gmailGoToAction

import (
	"encoding/json"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type potentialAction struct {
	Type   string `json:"@type"`
	Target string `json:"target"`
	URL    string `json:"url"`
	Name   string `json:"name"`
}

type fullGmailGoToAction struct {
	Context         string          `json:"@context"`
	Type            string          `json:"@type"`
	PotentialAction potentialAction `json:"potentialAction"`
	Description     string          `json:"description"`
}

type GmailGoToAction struct {
	Target      *sharedTypes.URL
	Name        string
	Description string
}

func (g *GmailGoToAction) MarshalJSON() ([]byte, error) {
	body := &fullGmailGoToAction{
		Context:     "http://schema.org",
		Description: g.Description,
		PotentialAction: potentialAction{
			Name:   g.Name,
			Target: g.Target.String(),
			Type:   "ViewAction",
			URL:    g.Target.String(),
		},
		Type: "EmailMessage",
	}

	b, err := json.Marshal(body)
	if err != nil {
		return nil, errors.Tag(err, "cannot marshal gmail action")
	}
	return b, nil
}
