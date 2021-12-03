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
	"html/template"
	"net/smtp"
	"time"

	"github.com/das7pad/overleaf-go/pkg/email"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/options/jwtOptions"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/pkg/signedCookie"
	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
	docstoreTypes "github.com/das7pad/overleaf-go/services/docstore/pkg/types"
	documentUpdaterTypes "github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
	filestoreTypes "github.com/das7pad/overleaf-go/services/filestore/pkg/types"
)

type Options struct {
	AdminEmail    sharedTypes.Email     `json:"admin_email"`
	AllowedImages []clsiTypes.ImageName `json:"allowed_images"`
	AppName       string                `json:"app_name"`
	DefaultImage  clsiTypes.ImageName   `json:"default_image"`
	Email         struct {
		CustomFooter     string            `json:"custom_footer"`
		CustomFooterHTML template.HTML     `json:"custom_footer_html"`
		From             *email.Identity   `json:"from"`
		FallbackReplyTo  *email.Identity   `json:"fallback_reply_to"`
		SMTPAddress      email.SMTPAddress `json:"smtp_address"`
		SMTPUser         string            `json:"smtp_user"`
		SMTPPassword     string            `json:"smtp_password"`
	} `json:"email"`
	PDFDownloadDomain        PDFDownloadDomain   `json:"pdf_download_domain"`
	SiteURL                  sharedTypes.URL     `json:"site_url"`
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
			URL     sharedTypes.URL         `json:"url"`
			Options *filestoreTypes.Options `json:"options"`
		} `json:"filestore"`
		LinkedURLProxy struct {
			Chain []sharedTypes.URL `json:"chain"`
		} `json:"linked_url_proxy"`
	} `json:"apis"`

	JWT struct {
		Compile      jwtOptions.JWTOptions `json:"compile"`
		LoggedInUser jwtOptions.JWTOptions `json:"logged_in_user"`
		Spelling     jwtOptions.JWTOptions `json:"spelling"`
		RealTime     jwtOptions.JWTOptions `json:"realTime"`
	} `json:"jwt"`

	SessionCookie signedCookie.Options `json:"session_cookie"`
}

func (o *Options) Validate() error {
	if err := o.AdminEmail.Validate(); err != nil {
		return errors.Tag(err, "admin_email is invalid")
	}
	if len(o.AllowedImages) == 0 {
		return &errors.ValidationError{Msg: "allowed_images is missing"}
	}
	if o.AppName == "" {
		return &errors.ValidationError{Msg: "app_name is missing"}
	}
	if len(o.DefaultImage) == 0 {
		return &errors.ValidationError{Msg: "default_image is missing"}
	}
	if err := o.SiteURL.Validate(); err != nil {
		return errors.Tag(err, "site_url is invalid")
	}

	if o.Email.From == nil {
		return &errors.ValidationError{Msg: "email.from is missing"}
	}
	if o.Email.FallbackReplyTo == nil {
		return &errors.ValidationError{
			Msg: "email.fallback_reply_to is missing",
		}
	}
	if o.Email.SMTPAddress == "" {
		return &errors.ValidationError{Msg: "email.smtp_address is missing"}
	}
	if err := o.Email.SMTPAddress.Validate(); err != nil {
		return errors.Tag(err, "email.smtp_address is invalid")
	}
	if o.Email.SMTPUser == "" {
		return &errors.ValidationError{Msg: "email.smtp_user is missing"}
	}
	if o.Email.SMTPPassword == "" {
		return &errors.ValidationError{Msg: "email.smtp_password is missing"}
	}

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
	if len(o.APIs.LinkedURLProxy.Chain) < 1 {
		return &errors.ValidationError{
			Msg: "api.linked_url_proxy.chain is too short",
		}
	}

	if err := o.JWT.Compile.Validate(); err != nil {
		return errors.Tag(err, "jwt.compile is invalid")
	}
	if err := o.JWT.LoggedInUser.Validate(); err != nil {
		return errors.Tag(err, "jwt.logged_in_user is invalid")
	}
	if err := o.JWT.RealTime.Validate(); err != nil {
		return errors.Tag(err, "jwt.realTime is invalid")
	}
	if err := o.JWT.Spelling.Validate(); err != nil {
		return errors.Tag(err, "jwt.spelling is invalid")
	}

	if err := o.SessionCookie.Validate(); err != nil {
		return errors.Tag(err, "session_cookie is invalid")
	}
	return nil
}

type EmailOptions struct {
	Public *email.PublicOptions
	Send   *email.SendOptions
}

func (o *Options) EmailOptions() *EmailOptions {
	return &EmailOptions{
		Public: &email.PublicOptions{
			AppName:          o.AppName,
			CustomFooter:     o.Email.CustomFooter,
			CustomFooterHTML: o.Email.CustomFooterHTML,
			SiteURL:          o.SiteURL.String(),
		},
		Send: &email.SendOptions{
			From:            o.Email.From,
			FallbackReplyTo: o.Email.FallbackReplyTo,
			SMTPAddress:     o.Email.SMTPAddress,
			SMTPAuth: smtp.PlainAuth(
				"",
				o.Email.SMTPUser,
				o.Email.SMTPPassword,
				o.Email.SMTPAddress.Host(),
			),
		},
	}
}
