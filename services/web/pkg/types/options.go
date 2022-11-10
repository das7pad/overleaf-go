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

package types

import (
	"html/template"
	"net/smtp"
	"strings"
	"time"

	"github.com/das7pad/overleaf-go/pkg/assets"
	"github.com/das7pad/overleaf-go/pkg/csp"
	"github.com/das7pad/overleaf-go/pkg/email"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/objectStorage"
	"github.com/das7pad/overleaf-go/pkg/options/env"
	"github.com/das7pad/overleaf-go/pkg/options/jwtOptions"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/pkg/signedCookie"
	"github.com/das7pad/overleaf-go/pkg/templates"
)

type Options struct {
	AdminEmail        sharedTypes.Email            `json:"admin_email"`
	AllowedImages     []sharedTypes.ImageName      `json:"allowed_images"`
	AllowedImageNames []templates.AllowedImageName `json:"allowed_image_names"`
	AppName           string                       `json:"app_name"`
	BcryptCost        int                          `json:"bcrypt_cost"`
	CDNURL            sharedTypes.URL              `json:"cdn_url"`
	CSPReportURL      *sharedTypes.URL             `json:"csp_report_url"`
	DefaultImage      sharedTypes.ImageName        `json:"default_image"`
	Email             struct {
		CustomFooter     string            `json:"custom_footer"`
		CustomFooterHTML template.HTML     `json:"custom_footer_html"`
		From             email.Identity    `json:"from"`
		FallbackReplyTo  email.Identity    `json:"fallback_reply_to"`
		SMTPAddress      email.SMTPAddress `json:"smtp_address"`
		SMTPHello        string            `json:"smtp_hello"`
		SMTPUser         string            `json:"smtp_user"`
		SMTPPassword     string            `json:"smtp_password"`
	} `json:"email"`
	I18n                templates.I18nOptions `json:"i18n"`
	LearnCacheDuration  time.Duration         `json:"learn_cache_duration"`
	LearnImageCacheBase sharedTypes.DirName   `json:"learn_image_cache_base"`
	ManifestPath        string                `json:"manifest_path"`
	Nav                 templates.NavOptions  `json:"nav"`
	PDFDownloadDomain   PDFDownloadDomain     `json:"pdf_download_domain"`
	Sentry              SentryOptions         `json:"sentry"`
	SiteURL             sharedTypes.URL       `json:"site_url"`
	SmokeTest           struct {
		Email     sharedTypes.Email `json:"email"`
		Password  UserPassword      `json:"password"`
		ProjectId sharedTypes.UUID  `json:"projectId"`
		UserId    sharedTypes.UUID  `json:"userId"`
	} `json:"smoke_test"`
	StatusPageURL             *sharedTypes.URL      `json:"status_page_url"`
	TeXLiveImageNameOverride  sharedTypes.ImageName `json:"texlive_image_name_override"`
	EmailConfirmationDisabled bool                  `json:"email_confirmation_disabled"`
	RegistrationDisabled      bool                  `json:"registration_disabled"`
	RobotsNoindex             bool                  `json:"robots_noindex"`
	WatchManifest             bool                  `json:"watch_manifest"`

	APIs struct {
		Clsi struct {
			URL         sharedTypes.URL `json:"url"`
			Persistence struct {
				CookieName string        `json:"cookie_name"`
				TTL        time.Duration `json:"ttl"`
			} `json:"persistence"`
		} `json:"clsi"`
		Filestore      objectStorage.Options `json:"filestore"`
		LinkedURLProxy struct {
			Chain []sharedTypes.URL `json:"chain"`
		} `json:"linked_url_proxy"`
	} `json:"apis"`

	JWT struct {
		Compile      jwtOptions.JWTOptions `json:"compile"`
		LoggedInUser jwtOptions.JWTOptions `json:"logged_in_user"`
		RealTime     jwtOptions.JWTOptions `json:"realTime"`
	} `json:"jwt"`

	SessionCookie signedCookie.Options `json:"session_cookie"`

	RateLimits struct {
		LinkSharingTokenLookupConcurrency int64 `json:"link_sharing_token_lookup_concurrency"`
	} `json:"rate_limits"`
}

func (o *Options) FillFromEnv() {
	env.MustParseJSON(o, "WEB_OPTIONS")
	o.JWT.Compile.FillFromEnv("JWT_WEB_VERIFY_SECRET")
	o.JWT.LoggedInUser.FillFromEnv("JWT_WEB_VERIFY_SECRET")
	o.JWT.RealTime.FillFromEnv("JWT_REAL_TIME_VERIFY_SECRET")
	o.SessionCookie.FillFromEnv("SESSION_SECRET")
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
	if o.BcryptCost < 10 {
		return &errors.ValidationError{Msg: "bcrypt_cost is too low"}
	}
	if err := o.CDNURL.Validate(); err != nil {
		return errors.Tag(err, "cdn_url is invalid")
	}
	if !strings.HasSuffix(o.CDNURL.Path, "/") {
		return &errors.ValidationError{Msg: `cdn_url must end with "/"`}
	}
	if len(o.DefaultImage) == 0 {
		return &errors.ValidationError{Msg: "default_image is missing"}
	}
	if err := o.I18n.Validate(); err != nil {
		return errors.Tag(err, "i18n is invalid")
	}
	if o.LearnCacheDuration < time.Second {
		return &errors.ValidationError{Msg: "learn_cache_duration is too low"}
	}
	if o.LearnImageCacheBase == "" {
		return &errors.ValidationError{
			Msg: "learn_image_cache_base is missing",
		}
	}
	if o.ManifestPath == "" {
		return &errors.ValidationError{
			Msg: "manifest_path is missing, use 'cdn' for download at boot",
		}
	}
	if err := o.SiteURL.Validate(); err != nil {
		return errors.Tag(err, "site_url is invalid")
	}

	if err := o.SmokeTest.Email.Validate(); err != nil {
		return errors.Tag(err, "smoke_test.email is invalid")
	}
	if err := o.SmokeTest.Password.Validate(); err != nil {
		return errors.Tag(err, "smoke_test.password is invalid")
	}
	if o.SmokeTest.ProjectId == (sharedTypes.UUID{}) {
		return &errors.ValidationError{Msg: "smoke_test.projectId is missing"}
	}
	if o.SmokeTest.UserId == (sharedTypes.UUID{}) {
		return &errors.ValidationError{Msg: "smoke_test.userId is missing"}
	}

	if o.Email.From.Address == "" {
		return &errors.ValidationError{Msg: "email.from is missing"}
	}
	if o.Email.FallbackReplyTo.Address == "" {
		return &errors.ValidationError{
			Msg: "email.fallback_reply_to is missing",
		}
	}
	if o.Email.SMTPAddress == "" {
		return &errors.ValidationError{
			Msg: "email.smtp_address is missing, use 'log' as no-op",
		}
	}
	if err := o.Email.SMTPAddress.Validate(); err != nil {
		return errors.Tag(err, "email.smtp_address is invalid")
	}
	if o.Email.SMTPAddress != "log" {
		if o.Email.SMTPUser == "" {
			return &errors.ValidationError{Msg: "email.smtp_user is missing"}
		}
		if o.Email.SMTPPassword == "" {
			return &errors.ValidationError{
				Msg: "email.smtp_password is missing",
			}
		}
	}

	if err := o.APIs.Clsi.URL.Validate(); err != nil {
		return errors.Tag(err, "apis.clsi.url is invalid")
	}
	if o.APIs.Clsi.Persistence.CookieName != "" {
		if o.APIs.Clsi.Persistence.TTL <= 0 {
			return &errors.ValidationError{
				Msg: "apis.clsi.persistence.ttl must be greater than zero",
			}
		}
	}
	if err := o.APIs.Filestore.Validate(); err != nil {
		return errors.Tag(err, "apis.filestore is invalid")
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

	if err := o.SessionCookie.Validate(); err != nil {
		return errors.Tag(err, "session_cookie is invalid")
	}

	if o.RateLimits.LinkSharingTokenLookupConcurrency < 1 {
		return errors.Tag(&errors.ValidationError{
			Msg: "link_sharing_token_lookup_concurrency must be at least 1",
		}, "rate_limits is invalid")
	}
	return nil
}

func (o *Options) AssetsOptions() assets.Options {
	return assets.Options{
		SiteURL:       o.SiteURL,
		CDNURL:        o.CDNURL,
		ManifestPath:  o.ManifestPath,
		WatchManifest: o.WatchManifest,
	}
}

type EmailOptions struct {
	Public *email.PublicOptions
	Send   *email.SendOptions
}

func (o *Options) EmailOptions() *EmailOptions {
	var a smtp.Auth
	if o.Email.SMTPAddress != "log" {
		a = smtp.PlainAuth(
			"",
			o.Email.SMTPUser,
			o.Email.SMTPPassword,
			o.Email.SMTPAddress.Host(),
		)
	}
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
			SMTPAuth:        a,
			SMTPHello:       o.Email.SMTPHello,
		},
	}
}

type SentryOptions struct {
	Frontend templates.SentryFrontendOptions `json:"frontend"`
}

func (o *Options) PublicSettings() (*templates.PublicSettings, error) {
	var sentryDSN, pdfDownloadDomain *sharedTypes.URL
	if dsn := o.Sentry.Frontend.Dsn; dsn != "" {
		u, err := sharedTypes.ParseAndValidateURL(dsn)
		if err != nil {
			return nil, errors.Tag(err, "sentry.frontend.dsn is invalid")
		}
		sentryDSN = u
	}
	if d := o.PDFDownloadDomain; d != "" {
		u, err := sharedTypes.ParseAndValidateURL(string(d))
		if err != nil {
			return nil, errors.Tag(err, "pdf_download_domain is invalid")
		}
		pdfDownloadDomain = u
	}

	//goland:noinspection SpellCheckingInspection
	return &templates.PublicSettings{
		AppName:    o.AppName,
		AdminEmail: o.AdminEmail,
		CDNURL:     o.CDNURL,
		CSPs: csp.Generate(csp.Options{
			CDNURL:            o.CDNURL,
			PdfDownloadDomain: pdfDownloadDomain,
			ReportURL:         o.CSPReportURL,
			SentryDSN:         sentryDSN,
			SiteURL:           o.SiteURL,
		}),
		EditorSettings: templates.EditorSettings{
			MaxDocLength:           sharedTypes.MaxDocLength,
			MaxEntitiesPerProject:  2000,
			MaxUploadSize:          MaxUploadSize,
			WikiEnabled:            true,
			WsURL:                  "/socket.io",
			WsRetryHandshake:       5,
			EnablePdfCaching:       false,
			ResetServiceWorker:     false,
			EditorThemes:           user.EditorThemes,
			TextExtensions:         sharedTypes.ValidTextExtensions,
			ValidRootDocExtensions: sharedTypes.ValidRootDocExtensions,
		},
		EmailConfirmationDisabled: o.EmailConfirmationDisabled,
		I18n:                      o.I18n,
		Nav:                       o.Nav,
		RegistrationDisabled:      o.RegistrationDisabled,
		RobotsNoindex:             o.RobotsNoindex,
		Sentry: templates.PublicSentryOptions{
			Frontend: o.Sentry.Frontend,
		},
		SiteURL:       o.SiteURL,
		StatusPageURL: o.StatusPageURL,
		TranslatedLanguages: map[string]string{
			"cn":    "简体中文",
			"cs":    "Čeština",
			"da":    "Dansk",
			"de":    "Deutsch",
			"en":    "English",
			"es":    "Español",
			"fi":    "Suomi",
			"fr":    "Français",
			"it":    "Italiano",
			"ja":    "日本語",
			"ko":    "한국어",
			"nl":    "Nederlands",
			"no":    "Norsk",
			"pl":    "Polski",
			"pt":    "Português",
			"ro":    "Română",
			"ru":    "Русский",
			"sv":    "Svenska",
			"tr":    "Türkçe",
			"uk":    "Українська",
			"zh-CN": "简体中文",
		},
		ZipFileSizeLimit: MaxUploadSize,
	}, nil
}
