// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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
	"github.com/das7pad/overleaf-go/pkg/constants"
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
	APIs                      ApisOptions                  `json:"apis"`
	AdminEmail                sharedTypes.Email            `json:"admin_email"`
	AllowedImageNames         []templates.AllowedImageName `json:"allowed_image_names"`
	AllowedImages             []sharedTypes.ImageName      `json:"allowed_images"`
	AppName                   string                       `json:"app_name"`
	BcryptCost                int                          `json:"bcrypt_cost"`
	CDNURL                    sharedTypes.URL              `json:"cdn_url"`
	CSPReportURL              *sharedTypes.URL             `json:"csp_report_url"`
	DefaultImage              sharedTypes.ImageName        `json:"default_image"`
	Email                     EmailOptionsRaw              `json:"email"`
	EmailConfirmationDisabled bool                         `json:"email_confirmation_disabled"`
	I18n                      templates.I18nOptions        `json:"i18n"`
	JWT                       JWTOptions                   `json:"jwt"`
	LearnCacheDuration        time.Duration                `json:"learn_cache_duration"`
	LearnImageCacheBase       sharedTypes.DirName          `json:"learn_image_cache_base"`
	ManifestPath              string                       `json:"manifest_path"`
	Nav                       templates.NavOptions         `json:"nav"`
	PDFDownloadDomain         PDFDownloadDomain            `json:"pdf_download_domain"`
	RateLimits                RateLimitsOptions            `json:"rate_limits"`
	RegistrationDisabled      bool                         `json:"registration_disabled"`
	RobotsNoindex             bool                         `json:"robots_noindex"`
	Sentry                    SentryOptions                `json:"sentry"`
	SessionCookie             signedCookie.Options         `json:"session_cookie"`
	SiteURL                   sharedTypes.URL              `json:"site_url"`
	SmokeTest                 SmokeTestOptions             `json:"smoke_test"`
	StatusPageURL             *sharedTypes.URL             `json:"status_page_url"`
	TeXLiveImageNameOverride  sharedTypes.ImageName        `json:"texlive_image_name_override"`
	WatchManifest             bool                         `json:"watch_manifest"`
}

type RateLimitsOptions struct {
	LinkSharingTokenLookupConcurrency int64 `json:"link_sharing_token_lookup_concurrency"`
}

func (o *RateLimitsOptions) Validate() error {
	if o.LinkSharingTokenLookupConcurrency < 1 {
		return &errors.ValidationError{
			Msg: "link_sharing_token_lookup_concurrency must be at least 1",
		}
	}
	return nil
}

type JWTOptions struct {
	Project      jwtOptions.JWTOptions `json:"project"`
	LoggedInUser jwtOptions.JWTOptions `json:"logged_in_user"`
}

func (o JWTOptions) Validate() error {
	if err := o.Project.Validate(); err != nil {
		return errors.Tag(err, "project")
	}
	if err := o.LoggedInUser.Validate(); err != nil {
		return errors.Tag(err, "logged_in_user")
	}
	return nil
}

type EmailOptionsRaw struct {
	CustomFooter     string            `json:"custom_footer"`
	CustomFooterHTML template.HTML     `json:"custom_footer_html"`
	From             email.Identity    `json:"from"`
	FallbackReplyTo  email.Identity    `json:"fallback_reply_to"`
	SMTPAddress      email.SMTPAddress `json:"smtp_address"`
	SMTPHello        string            `json:"smtp_hello"`
	SMTPIdentity     string            `json:"smtp_identity"`
	SMTPUser         string            `json:"smtp_user"`
	SMTPPassword     string            `json:"smtp_password"`
}

func (o *EmailOptionsRaw) Validate() error {
	if o.From.Address == "" {
		return &errors.ValidationError{Msg: "from is missing"}
	}
	if o.FallbackReplyTo.Address == "" {
		return &errors.ValidationError{Msg: "fallback_reply_to is missing"}
	}
	if o.SMTPAddress == "" {
		return &errors.ValidationError{
			Msg: "smtp_address is missing, use 'discard'/'log' as no-op",
		}
	}
	if err := o.SMTPAddress.Validate(); err != nil {
		return errors.Tag(err, "smtp_address")
	}
	if !o.SMTPAddress.IsSpecial() {
		if o.SMTPUser == "" {
			return &errors.ValidationError{Msg: "smtp_user is missing"}
		}
		if o.SMTPPassword == "" {
			return &errors.ValidationError{Msg: "smtp_password is missing"}
		}
	}
	return nil
}

type ClsiPersistenceOptions struct {
	CookieName string        `json:"cookie_name"`
	TTL        time.Duration `json:"ttl"`
}

func (o ClsiPersistenceOptions) Validate() error {
	if o.CookieName == "" {
		// persistence disabled
		return nil
	}
	if o.TTL <= 0 {
		return &errors.ValidationError{Msg: "ttl must be greater than zero"}
	}
	return nil
}

type ClsiOptions struct {
	URL         sharedTypes.URL        `json:"url"`
	Persistence ClsiPersistenceOptions `json:"persistence"`
}

func (o *ClsiOptions) Validate() error {
	if err := o.URL.Validate(); err != nil {
		return errors.Tag(err, "url")
	}
	if err := o.Persistence.Validate(); err != nil {
		return errors.Tag(err, "persistence")
	}
	return nil
}

type LinkedURLProxyOptions struct {
	Chain []sharedTypes.URL `json:"chain"`
}

func (o *LinkedURLProxyOptions) Validate() error {
	if len(o.Chain) < 1 {
		return &errors.ValidationError{Msg: "chain is too short"}
	}
	return nil
}

type ApisOptions struct {
	Clsi           ClsiOptions           `json:"clsi"`
	Filestore      objectStorage.Options `json:"filestore"`
	LinkedURLProxy LinkedURLProxyOptions `json:"linked_url_proxy"`
}

func (o *ApisOptions) Validate() error {
	if err := o.Clsi.Validate(); err != nil {
		return errors.Tag(err, "clsi")
	}
	if err := o.Filestore.Validate(); err != nil {
		return errors.Tag(err, "filestore")
	}
	if err := o.LinkedURLProxy.Validate(); err != nil {
		return errors.Tag(err, "linked_url_proxy")
	}
	return nil
}

type SmokeTestOptions struct {
	Email     sharedTypes.Email `json:"email"`
	Password  UserPassword      `json:"password"`
	ProjectId sharedTypes.UUID  `json:"projectId"`
	UserId    sharedTypes.UUID  `json:"userId"`
}

func (o *SmokeTestOptions) Validate() error {
	if err := o.Email.Validate(); err != nil {
		return errors.Tag(err, "email")
	}
	if err := o.Password.Validate(); err != nil {
		return errors.Tag(err, "password")
	}
	if o.ProjectId.IsZero() {
		return &errors.ValidationError{Msg: "projectId is missing"}
	}
	if o.UserId.IsZero() {
		return &errors.ValidationError{Msg: "userId is missing"}
	}
	return nil
}

func (o *Options) FillFromEnv() {
	env.MustParseJSON(o, "WEB_OPTIONS")
	o.JWT.Project.FillFromEnv("JWT_WEB_VERIFY_SECRET")
	o.JWT.LoggedInUser.FillFromEnv("JWT_WEB_VERIFY_SECRET")
	o.SessionCookie.FillFromEnv("SESSION_SECRET")
}

func (o *Options) Validate() error {
	isTesting := o.SiteURL.Host == "localhost:8080" &&
		o.ManifestPath == "empty" &&
		o.Email.SMTPAddress == "discard" &&
		o.AppName == "TESTING"

	if err := o.AdminEmail.Validate(); err != nil {
		return errors.Tag(err, "admin_email")
	}
	if len(o.AllowedImages) == 0 {
		return &errors.ValidationError{Msg: "allowed_images is missing"}
	}
	if o.AppName == "" {
		return &errors.ValidationError{Msg: "app_name is missing"}
	}
	if o.BcryptCost < 10 && !isTesting {
		return &errors.ValidationError{Msg: "bcrypt_cost is too low"}
	}
	if err := o.CDNURL.Validate(); err != nil {
		return errors.Tag(err, "cdn_url")
	}
	if !strings.HasSuffix(o.CDNURL.Path, "/") {
		return &errors.ValidationError{Msg: `cdn_url must end with "/"`}
	}
	if len(o.DefaultImage) == 0 {
		return &errors.ValidationError{Msg: "default_image is missing"}
	}
	if err := o.I18n.Validate(); err != nil {
		return errors.Tag(err, "i18n")
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
		return errors.Tag(err, "site_url")
	}
	if err := o.SmokeTest.Validate(); err != nil {
		return errors.Tag(err, "smoke_test")
	}
	if err := o.Email.Validate(); err != nil {
		return errors.Tag(err, "email")
	}
	if err := o.APIs.Validate(); err != nil {
		return errors.Tag(err, "apis")
	}
	if err := o.JWT.Validate(); err != nil {
		return errors.Tag(err, "jwt")
	}
	if err := o.SessionCookie.Validate(); err != nil {
		return errors.Tag(err, "session_cookie")
	}
	if err := o.RateLimits.Validate(); err != nil {
		return errors.Tag(err, "rate_limits")
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
	if !o.Email.SMTPAddress.IsSpecial() {
		a = smtp.PlainAuth(
			o.Email.SMTPIdentity,
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
			Sender:          email.NewSender(o.Email.SMTPAddress, o.Email.SMTPHello, a),
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
			return nil, errors.Tag(err, "sentry.frontend.dsn")
		}
		sentryDSN = u
	}
	if d := o.PDFDownloadDomain; d != "" {
		u, err := sharedTypes.ParseAndValidateURL(string(d))
		if err != nil {
			return nil, errors.Tag(err, "pdf_download_domain")
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
			MaxUploadSize:          constants.MaxUploadSize,
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
		ZipFileSizeLimit: constants.MaxUploadSize,
	}, nil
}
