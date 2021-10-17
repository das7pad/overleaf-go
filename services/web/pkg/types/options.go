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
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/options/jwtOptions"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
	docstoreTypes "github.com/das7pad/overleaf-go/services/docstore/pkg/types"
	documentUpdaterTypes "github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
)

type Options struct {
	PDFDownloadDomain        PDFDownloadDomain   `json:"pdf_download_domain"`
	TeXLiveImageNameOverride clsiTypes.ImageName `json:"texlive_image_name_override"`

	APIs struct {
		Clsi struct {
			URL         sharedTypes.URL `json:"url"`
			Persistence struct {
				CookieName string        `json:"cookie_name"`
				TTL        time.Duration `json:"ttl"`
			} `json:"persistence"`
		} `json:"clsi"`
		Docstore struct {
			Options *docstoreTypes.Options `json:"options"`
		} `json:"docstore"`
		DocumentUpdater struct {
			Options *documentUpdaterTypes.Options `json:"options"`
		} `json:"document_updater"`
		Filestore struct {
			URL sharedTypes.URL `json:"url"`
		} `json:"filestore"`
	} `json:"apis"`

	JWT struct {
		Compile       jwtOptions.JWTOptions `json:"compile"`
		Notifications jwtOptions.JWTOptions `json:"notifications"`
		Spelling      jwtOptions.JWTOptions `json:"spelling"`
		RealTime      jwtOptions.JWTOptions `json:"realTime"`
	} `json:"jwt"`
}

func (o *Options) Validate() error {
	if err := o.APIs.Clsi.URL.Validate(); err != nil {
		return errors.Tag(err, "apis.clsi.url is invalid")
	}
	if o.APIs.Clsi.Persistence.TTL <= 0 {
		return &errors.ValidationError{
			Msg: "apis.clsi.persistence.ttl must be greater than zero",
		}
	}
	if err := o.APIs.Docstore.Options.Validate(); err != nil {
		return errors.Tag(err, "apis.docstore.options is invalid")
	}
	if err := o.APIs.DocumentUpdater.Options.Validate(); err != nil {
		return errors.Tag(err, "apis.document_updater.options is invalid")
	}
	if err := o.APIs.Filestore.URL.Validate(); err != nil {
		return errors.Tag(err, "apis.filestore.url is invalid")
	}
	return nil
}
