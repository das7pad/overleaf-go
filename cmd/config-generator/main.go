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

package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"os"
	"path"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/das7pad/overleaf-go/pkg/email"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/options/jwtOptions"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/pkg/signedCookie"
	"github.com/das7pad/overleaf-go/pkg/templates"
	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
	documentUpdaterTypes "github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
	"github.com/das7pad/overleaf-go/services/filestore/pkg/types"
	realTimeTypes "github.com/das7pad/overleaf-go/services/real-time/pkg/types"
	spellingTypes "github.com/das7pad/overleaf-go/services/spelling/pkg/types"
	webTypes "github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func genSecret(n int) string {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		panic(errors.Tag(err, "generate secret"))
	}
	return hex.EncodeToString(b)
}

func serialize(d interface{}, label string) string {
	blob, err := json.Marshal(d)
	if err != nil {
		panic(errors.Tag(err, "serialize "+label))
	}
	return string(blob)
}

func main() {
	defer func() {
		err := recover()
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "Error:", err)
			flag.CommandLine.Usage()
			os.Exit(101)
		}
	}()
	jwtOptionsRealTime := jwtOptions.JWTOptions{
		Algorithm: "HS512",
		Key:       []byte(genSecret(32)),
		ExpiresIn: 30 * time.Second,
	}
	jwtOptionsLoggedInUser := jwtOptions.JWTOptions{
		Algorithm: "HS512",
		Key:       []byte(genSecret(32)),
		ExpiresIn: 24 * time.Hour,
	}
	jwtOptionsProject := jwtOptions.JWTOptions{
		Algorithm: "HS512",
		Key:       []byte(genSecret(32)),
		ExpiresIn: time.Hour,
	}

	dockerSocket := "unix:///var/run/docker.sock"
	flag.StringVar(&dockerSocket, "docker-socket", dockerSocket, "docker socket path")

	dockerContainerUser := "tex"
	flag.StringVar(&dockerContainerUser, "texlive-container-user", dockerContainerUser, "user inside the docker container running texlive")

	dockerRootless := false
	flag.BoolVar(&dockerRootless, "docker-rootless", dockerRootless, "run in rootless docker environment")

	texLiveImages := "texlive/texlive:TL2021-historic"
	flag.StringVar(&texLiveImages, "texlive-images", texLiveImages, "comma separated list of texlive docker images, first image is default image")

	tmpDir := "/tmp/overleaf"
	flag.StringVar(&tmpDir, "tmp-dir", tmpDir, "base dir for ephemeral files")

	manifestPath := ""
	flag.StringVar(&manifestPath, "frontend-manifest-path", manifestPath, "frontend manifest path")

	siteURLRaw := ""
	flag.StringVar(&siteURLRaw, "site-url", siteURLRaw, "site url")

	cdnURLRaw := ""
	flag.StringVar(&cdnURLRaw, "cdn-url", cdnURLRaw, "cdn url")

	clsiUrlRaw := ""
	flag.StringVar(&clsiUrlRaw, "clsi-url", clsiUrlRaw, "clsi url (required when running separate processes)")

	pdfDownloadURLRaw := ""
	flag.StringVar(&pdfDownloadURLRaw, "pdf-download-url", pdfDownloadURLRaw, "pdf download url")

	clsiCookieName := ""
	flag.StringVar(&clsiCookieName, "clsi-cookie-name", clsiCookieName, "clsi cookie name (required when load balancing compiles)")

	sessionCookieName := "ol.go"
	flag.StringVar(&sessionCookieName, "session-cookie-name", sessionCookieName, "session cookie name")

	smtpAddress := ""
	flag.StringVar(&smtpAddress, "email-smtp-address", smtpAddress, "address:port of email provider")

	smtpUser := ""
	flag.StringVar(&smtpUser, "email-smtp-user", smtpUser, "login user name at email provider")

	smtpPassword := "-"
	flag.StringVar(&smtpPassword, "email-smtp-password", smtpPassword, "login password at email provider, use '-' for prompt")

	flag.Parse()

	if smtpPassword != "-" {
		_, _ = fmt.Fprintf(os.Stderr, "Enter SMTP Password: ")
		s, err := term.ReadPassword(int(os.Stdin.Fd()))
		_, _ = fmt.Fprintln(os.Stderr, "")
		if err != nil {
			panic(errors.Tag(err, "read smtp password"))
		}
		smtpPassword = string(s)
	}

	var siteURL sharedTypes.URL
	{
		u, err := sharedTypes.ParseAndValidateURL(siteURLRaw)
		if err != nil {
			panic(errors.Tag(err, "site-url"))
		}
		siteURL = *u
	}

	var cdnURL sharedTypes.URL
	{
		u, err := sharedTypes.ParseAndValidateURL(cdnURLRaw)
		if err != nil {
			panic(errors.Tag(err, "cdn-url"))
		}
		cdnURL = *u
	}

	var clsiUrl sharedTypes.URL
	if clsiUrlRaw != "" {
		u, err := sharedTypes.ParseAndValidateURL(clsiUrlRaw)
		if err != nil {
			panic(errors.Tag(err, "clsi-url"))
		}
		clsiUrl = *u
	}

	if dockerRootless && dockerSocket == "unix:///var/run/docker.sock" {
		dockerContainerUser = "root"
		dockerSocket = fmt.Sprintf(
			"unix:///run/user/%d/docker.sock", os.Getuid(),
		)
	}
	var allowedImages []sharedTypes.ImageName
	var allowedImageNames []templates.AllowedImageName
	for _, s := range strings.Split(texLiveImages, ",") {
		allowedImages = append(allowedImages, sharedTypes.ImageName(s))
		allowedImageNames = append(allowedImageNames, templates.AllowedImageName{
			Name: sharedTypes.ImageName(s),
			Desc: s,
		})
	}
	agentPathHost := path.Join(tmpDir, "exec-agent")

	clsiOptions := clsiTypes.Options{
		AllowedImages:             allowedImages,
		CopyExecAgentDst:          agentPathHost,
		ProjectCacheDuration:      time.Hour,
		RefreshCapacityEvery:      5 * time.Second,
		RefreshHealthCheckEvery:   30 * time.Second,
		ParallelOutputWrite:       10,
		ParallelResourceWrite:     20,
		MaxFilesAndDirsPerProject: 4000,
		URLDownloadRetries:        3,
		URLDownloadTimeout:        10 * time.Second,
		Paths: clsiTypes.Paths{
			CacheBaseDir:   clsiTypes.CacheBaseDir(path.Join(tmpDir, "cache")),
			CompileBaseDir: clsiTypes.CompileBaseDir(path.Join(tmpDir, "compiles")),
			OutputBaseDir:  clsiTypes.OutputBaseDir(path.Join(tmpDir, "output")),
		},
		LatexBaseEnv: nil,
		Runner:       "agent",
		DockerContainerOptions: clsiTypes.DockerContainerOptions{
			User:                   dockerContainerUser,
			Env:                    nil,
			AgentPathContainer:     agentPathHost,
			AgentPathHost:          agentPathHost,
			AgentContainerLifeSpan: time.Hour,
			AgentRestartAttempts:   3,
			Runtime:                "",
			SeccompPolicyPath:      "",
			CompileBaseDir:         clsiTypes.CompileBaseDir(path.Join(tmpDir, "compiles")),
			OutputBaseDir:          clsiTypes.OutputBaseDir(path.Join(tmpDir, "output")),
		},
	}
	fmt.Printf("CLSI_OPTIONS=%s\n", serialize(clsiOptions, "clsi options"))

	documentUpdaterOptions := documentUpdaterTypes.Options{
		Workers:                      10,
		PendingUpdatesListShardCount: 1,
	}
	fmt.Printf("DOCUMENT_UPDATER_OPTIONS=%s\n", serialize(documentUpdaterOptions, "document updater options"))

	realTimeOptions := realTimeTypes.Options{
		GracefulShutdown: struct {
			Delay   time.Duration `json:"delay"`
			Timeout time.Duration `json:"timeout"`
		}{
			Delay:   3 * time.Second,
			Timeout: 10 * time.Second,
		},
		APIs: struct {
			DocumentUpdater struct {
				Options *documentUpdaterTypes.Options `json:"options"`
			} `json:"document_updater"`
		}{
			DocumentUpdater: struct {
				Options *documentUpdaterTypes.Options `json:"options"`
			}{
				Options: &documentUpdaterOptions,
			},
		},
		JWT: struct {
			RealTime jwtOptions.JWTOptions `json:"realTime"`
		}{
			RealTime: jwtOptionsRealTime,
		},
	}
	fmt.Printf("REAL_TIME_OPTIONS=%s\n", serialize(realTimeOptions, "realtime options"))

	spellingOptions := spellingTypes.Options{
		LRUSize: 10_000,
	}
	fmt.Printf("SPELLING_OPTIONS=%s\n", serialize(spellingOptions, "spelling options"))

	webOptions := webTypes.Options{
		AdminEmail:        sharedTypes.Email("support@" + siteURL.Host),
		AllowedImages:     allowedImages,
		AllowedImageNames: allowedImageNames,
		AppName:           "Overleaf Go",
		BcryptCost:        13,
		CDNURL:            cdnURL,
		CSPReportURL:      nil,
		DefaultImage:      allowedImages[0],
		Email: struct {
			CustomFooter     string            `json:"custom_footer"`
			CustomFooterHTML template.HTML     `json:"custom_footer_html"`
			From             email.Identity    `json:"from"`
			FallbackReplyTo  email.Identity    `json:"fallback_reply_to"`
			SMTPAddress      email.SMTPAddress `json:"smtp_address"`
			SMTPUser         string            `json:"smtp_user"`
			SMTPPassword     string            `json:"smtp_password"`
		}{
			From: email.Identity{
				Address: sharedTypes.Email("no-reply@" + siteURL.Host),
			},
			FallbackReplyTo: email.Identity{
				Address: sharedTypes.Email("support@" + siteURL.Host),
			},
			SMTPAddress:  email.SMTPAddress(smtpAddress),
			SMTPUser:     smtpUser,
			SMTPPassword: smtpPassword,
		},
		I18n: templates.I18nOptions{
			DefaultLang: "en",
			SubdomainLang: []templates.I18nSubDomainLang{
				{LngCode: "en"},
				{LngCode: "de"},
				{LngCode: "es"},
				{LngCode: "fr"},
			},
		},
		LearnCacheDuration:  31 * 24 * time.Hour,
		LearnImageCacheBase: sharedTypes.DirName(path.Join(tmpDir, "learn-images")),
		ManifestPath:        manifestPath,
		Nav:                 templates.NavOptions{},
		PDFDownloadDomain:   webTypes.PDFDownloadDomain(pdfDownloadURLRaw),
		Sentry:              webTypes.SentryOptions{},
		SiteURL:             siteURL,
		SmokeTest: struct {
			Email     sharedTypes.Email     `json:"email"`
			Password  webTypes.UserPassword `json:"password"`
			ProjectId sharedTypes.UUID      `json:"projectId"`
			UserId    sharedTypes.UUID      `json:"userId"`
		}{
			Email:     sharedTypes.Email("smoke-test@" + siteURL.Host),
			Password:  webTypes.UserPassword(genSecret(72 / 2)),
			ProjectId: sharedTypes.UUID{42},
			UserId:    sharedTypes.UUID{42},
		},
		StatusPageURL:             sharedTypes.URL{},
		TeXLiveImageNameOverride:  "",
		EmailConfirmationDisabled: false,
		RegistrationDisabled:      false,
		RobotsNoindex:             false,
		WatchManifest:             false,
		APIs: struct {
			Clsi struct {
				URL         sharedTypes.URL `json:"url"`
				Persistence struct {
					CookieName string        `json:"cookie_name"`
					TTL        time.Duration `json:"ttl"`
				} `json:"persistence"`
			} `json:"clsi"`
			DocumentUpdater struct {
				Options *documentUpdaterTypes.Options `json:"options"`
			} `json:"document_updater"`
			Filestore struct {
				Options *types.Options `json:"options"`
			} `json:"filestore"`
			LinkedURLProxy struct {
				Chain []sharedTypes.URL `json:"chain"`
			} `json:"linked_url_proxy"`
		}{
			Clsi: struct {
				URL         sharedTypes.URL `json:"url"`
				Persistence struct {
					CookieName string        `json:"cookie_name"`
					TTL        time.Duration `json:"ttl"`
				} `json:"persistence"`
			}{
				URL: clsiUrl,
				Persistence: struct {
					CookieName string        `json:"cookie_name"`
					TTL        time.Duration `json:"ttl"`
				}{
					CookieName: clsiCookieName,
					TTL:        6 * time.Hour,
				},
			},
			DocumentUpdater: struct {
				Options *documentUpdaterTypes.Options `json:"options"`
			}{
				Options: &documentUpdaterOptions,
			},
			Filestore: struct {
				Options *types.Options `json:"options"`
			}{},
			LinkedURLProxy: struct {
				Chain []sharedTypes.URL `json:"chain"`
			}{},
		},
		JWT: struct {
			Compile      jwtOptions.JWTOptions `json:"compile"`
			LoggedInUser jwtOptions.JWTOptions `json:"logged_in_user"`
			RealTime     jwtOptions.JWTOptions `json:"realTime"`
		}{
			Compile:      jwtOptionsProject,
			LoggedInUser: jwtOptionsLoggedInUser,
			RealTime:     jwtOptionsRealTime,
		},
		SessionCookie: signedCookie.Options{
			Domain:  siteURL.Host,
			Expiry:  7 * 24 * time.Hour,
			Name:    sessionCookieName,
			Path:    siteURL.Path,
			Secrets: []string{genSecret(32)},
		},
	}

	fmt.Printf("WEB_OPTIONS=%s\n", serialize(webOptions, "web options"))
}