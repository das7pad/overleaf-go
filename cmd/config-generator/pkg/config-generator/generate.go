// Golang port of Overleaf
// Copyright (C) 2022-2024 Jakob Ackermann <das7pad@outlook.com>
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

package configGenerator

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/das7pad/overleaf-go/pkg/email"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/objectStorage"
	"github.com/das7pad/overleaf-go/pkg/options/jwtOptions"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/pkg/signedCookie"
	"github.com/das7pad/overleaf-go/pkg/templates"
	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
	documentUpdaterTypes "github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
	realTimeTypes "github.com/das7pad/overleaf-go/services/real-time/pkg/types"
	spellingTypes "github.com/das7pad/overleaf-go/services/spelling/pkg/types"
	webTypes "github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func NewFlags() Flags {
	dockerSocketRootLess := fmt.Sprintf(
		"/run/user/%d/docker.sock", os.Getuid(),
	)
	dockerSocketRootful := "/var/run/docker.sock"
	dockerSocketRootfulMac := "/var/run/docker.sock.raw"
	if isSocket(dockerSocketRootfulMac) {
		dockerSocketRootful = dockerSocketRootfulMac
	}
	dockerSocket := dockerSocketRootful
	dockerRootlessAvailable := isSocket(dockerSocketRootLess)
	if dockerRootlessAvailable {
		dockerSocket = dockerSocketRootLess
	}

	return Flags{
		AppName:             "Overleaf Go",
		BcryptCosts:         13,
		CDNURLRaw:           "",
		CLSICookieName:      "",
		CLSIURLRaw:          "http://127.0.0.1:3013",
		CleanupOtherS3Keys:  true,
		DockerComposeSetup:  true,
		DockerContainerUser: "nobody",
		DockerSocket:        dockerSocket,
		DockerSocketGroupId: -1,
		DockerSocketRootful: dockerSocketRootful,
		FilestoreOptions: objectStorage.Options{
			Bucket:          "overleaf-files",
			Provider:        "minio",
			Endpoint:        "127.0.0.1:9000",
			Region:          "us-east-1",
			Secure:          false,
			Key:             genSecret(32),
			Secret:          genSecret(32),
			SignedURLExpiry: 15 * time.Minute,
		},
		JWTOptionsLoggedInUser: jwtOptions.JWTOptions{
			Algorithm: "HS256",
			Key:       genSecret(32),
			ExpiresIn: 24 * time.Hour,
		},
		JWTOptionsProject: jwtOptions.JWTOptions{
			Algorithm: "HS256",
			Key:       genSecret(32),
			ExpiresIn: time.Hour,
		},
		LinkedURLProxyChainRaw:   "",
		LinkedURLProxyToken:      genSecret(32),
		ManifestPath:             "",
		MinioRootPassword:        genSecret(32),
		MinioRootUser:            genSecret(32),
		NginxInternalPort:        "8080",
		NginxPublicListenAddress: "127.0.0.1:8080",
		PDFDownloadDomainRaw:     "",
		RealTimeWriteQueueDepth:  10,
		S3PolicyName:             "overleaf-go",
		SiteURLRaw:               "http://localhost:8080",
		SMTPAddress:              "log",
		SMTPPassword:             "-",
		SMTPUser:                 "",
		SessionCookieName:        "ol.go",
		SessionCookieSecretsRaw:  genSecret(32),
		SmokeTestUserPassword:    genSecret(72 / 2),
		TexLiveImagesRaw:         "texlive/texlive:TL2022-historic",
		TmpDir:                   "/tmp/ol",
	}
}

type Flags struct {
	AppName                  string
	BcryptCosts              int
	CDNURLRaw                string
	CLSICookieName           string
	CLSIURLRaw               string
	CleanupOtherS3Keys       bool
	DockerComposeSetup       bool
	DockerContainerUser      string
	DockerSocket             string
	DockerSocketGroupId      int
	DockerSocketRootful      string
	FilestoreOptions         objectStorage.Options
	JWTOptionsLoggedInUser   jwtOptions.JWTOptions
	JWTOptionsProject        jwtOptions.JWTOptions
	LinkedURLProxyChainRaw   string
	LinkedURLProxyToken      string
	ManifestPath             string
	MinioRootPassword        string
	MinioRootUser            string
	NginxInternalPort        string
	NginxPublicListenAddress string
	PDFDownloadDomainRaw     string
	RealTimeWriteQueueDepth  int
	S3PolicyName             string
	SMTPAddress              string
	SMTPPassword             string
	SMTPUser                 string
	SessionCookieName        string
	SessionCookieSecretsRaw  string
	SiteURLRaw               string
	SmokeTestUserEmail       string
	SmokeTestUserPassword    string
	TexLiveImagesRaw         string
	TmpDir                   string
}

func (f Flags) DockerHost() string {
	return fmt.Sprintf("unix://%s", f.DockerSocket)
}

type Config struct {
	CLSIOptions              clsiTypes.Options
	CleanupOtherS3Keys       bool
	DockerHost               string
	DockerSocket             string
	DockerSocketGroupId      int
	DockerUser               string
	DocumentUpdaterOptions   documentUpdaterTypes.Options
	FilestoreOptions         objectStorage.Options
	LinkedURLProxyToken      string
	MinioRootPassword        string
	MinioRootUser            string
	NginxInternalPort        string
	NginxPublicListenAddress string
	RealTimeOptions          realTimeTypes.Options
	S3PolicyContent          string
	S3PolicyName             string
	SiteURL                  sharedTypes.URL
	SpellingOptions          spellingTypes.Options
	TmpDir                   string
	WebOptions               webTypes.Options
}

func (c Config) PopulateEnv() {
	mustSetEnv("PUBLIC_URL", c.SiteURL.String())
	validateAndSetEnv("CLSI_OPTIONS", &c.CLSIOptions)
	mustSetEnv("DOCKER_HOST", c.DockerHost)
	validateAndSetEnv("DOCUMENT_UPDATER_OPTIONS", &c.DocumentUpdaterOptions)
	mustSetEnv("PROXY_TOKEN", c.LinkedURLProxyToken)
	validateAndSetEnv("REAL_TIME_OPTIONS", &c.RealTimeOptions)
	validateAndSetEnv("SPELLING_OPTIONS", &c.SpellingOptions)
	validateAndSetEnv("WEB_OPTIONS", &c.WebOptions)
}

func Generate(f Flags) Config {
	siteURL := mustParseURL("site-url", f.SiteURLRaw)

	if f.CDNURLRaw == "" {
		f.CDNURLRaw = siteURL.JoinPath("/assets/").String()
	}
	cdnURL := mustParseURL("cdn-url", f.CDNURLRaw)

	var clsiURL sharedTypes.URL
	if f.CLSIURLRaw != "" {
		clsiURL = mustParseURL("clsi-url", f.CLSIURLRaw)
	}

	if f.DockerComposeSetup && f.LinkedURLProxyChainRaw == "" {
		f.LinkedURLProxyChainRaw = "http://linked-url-proxy:8080/proxy/" + f.LinkedURLProxyToken
	}
	linkedURLProxyChain := make([]sharedTypes.URL, 0)
	for i, s := range strings.Split(f.LinkedURLProxyChainRaw, ",") {
		s = strings.Trim(s, `"' `)
		u := mustParseURL(fmt.Sprintf("linked-url-proxy-chain idx=%d", i), s)
		linkedURLProxyChain = append(linkedURLProxyChain, u)
	}

	rawImages := strings.Split(f.TexLiveImagesRaw, ",")
	allowedImages := make([]sharedTypes.ImageName, 0, len(rawImages))
	allowedImageNames := make([]templates.AllowedImageName, 0, len(rawImages))
	for i, s := range rawImages {
		imageName := sharedTypes.ImageName(s)
		if err := imageName.Validate(); err != nil {
			panic(errors.Tag(err, fmt.Sprintf("texlive-images idx=%d", i)))
		}
		allowedImages = append(allowedImages, imageName)
		allowedImageNames = append(allowedImageNames, templates.AllowedImageName{
			Name: imageName,
			Desc: imageName.Year(),
		})
	}
	agentPathHost := path.Join(f.TmpDir, "exec-agent")

	if f.ManifestPath == "" {
		f.ManifestPath = path.Join(f.TmpDir, "assets/manifest.json")
	}

	if f.DockerComposeSetup && f.FilestoreOptions.Endpoint == "127.0.0.1:9000" {
		f.FilestoreOptions.Endpoint = "minio:9000"
	}

	if f.DockerSocketGroupId == -1 {
		s := syscall.Stat_t{}
		if err := syscall.Stat(f.DockerSocketRootful, &s); err != nil {
			panic(errors.Tag(err, "detect docker group-id on docker socket"))
		}
		f.DockerSocketGroupId = int(s.Gid)
	}

	clsiOptions := clsiTypes.Options{
		AllowedImages:             allowedImages,
		CopyExecAgentDst:          agentPathHost,
		ProjectCacheDuration:      time.Hour,
		ProjectRunnerMaxAge:       time.Hour / 4,
		RefreshHealthCheckEvery:   30 * time.Second,
		ParallelOutputWrite:       10,
		ParallelResourceWrite:     20,
		MaxFilesAndDirsPerProject: 4000,
		URLDownloadRetries:        3,
		URLDownloadTimeout:        10 * time.Second,
		Paths: clsiTypes.Paths{
			CacheBaseDir:   clsiTypes.CacheBaseDir(path.Join(f.TmpDir, "cache")),
			CompileBaseDir: clsiTypes.CompileBaseDir(path.Join(f.TmpDir, "compiles")),
			OutputBaseDir:  clsiTypes.OutputBaseDir(path.Join(f.TmpDir, "output")),
		},
		LatexBaseEnv: nil,
		Runner:       "agent",
		DockerContainerOptions: clsiTypes.DockerContainerOptions{
			User:                 f.DockerContainerUser,
			Env:                  nil,
			AgentPathContainer:   agentPathHost,
			AgentPathHost:        agentPathHost,
			AgentRestartAttempts: 3,
			Runtime:              "",
			SeccompPolicyPath:    "",
			CompileBaseDir:       clsiTypes.CompileBaseDir(path.Join(f.TmpDir, "compiles")),
			OutputBaseDir:        clsiTypes.OutputBaseDir(path.Join(f.TmpDir, "output")),
		},
	}

	documentUpdaterOptions := documentUpdaterTypes.Options{
		Workers:                      20,
		PendingUpdatesListShardCount: 1,
		PeriodicFlushAll: struct {
			Count    int64         `json:"count"`
			Interval time.Duration `json:"interval"`
		}{
			Count:    10,
			Interval: 12 * time.Hour,
		},
	}

	realTimeOptions := realTimeTypes.Options{
		GracefulShutdown: realTimeTypes.GracefulShutdownOptions{
			Delay:          3 * time.Second,
			Timeout:        10 * time.Second,
			CleanupTimeout: 10 * time.Second,
		},
		WriteQueueDepth: f.RealTimeWriteQueueDepth,
		JWT: struct {
			Project jwtOptions.JWTOptions `json:"project"`
		}{
			Project: f.JWTOptionsProject,
		},
	}

	spellingOptions := spellingTypes.Options{
		LRUSize: 10_000,
	}

	emailHost := siteURL.Hostname()
	if f.SmokeTestUserEmail == "" {
		f.SmokeTestUserEmail = "smoke-test@" + emailHost
	}
	webOptions := webTypes.Options{
		AdminEmail:        sharedTypes.Email("support@" + emailHost),
		AllowedImages:     allowedImages,
		AllowedImageNames: allowedImageNames,
		AppName:           f.AppName,
		BcryptCost:        f.BcryptCosts,
		CDNURL:            cdnURL,
		CSPReportURL:      nil,
		DefaultImage:      allowedImages[0],
		Email: struct {
			CustomFooter     string            `json:"custom_footer"`
			CustomFooterHTML template.HTML     `json:"custom_footer_html"`
			From             email.Identity    `json:"from"`
			FallbackReplyTo  email.Identity    `json:"fallback_reply_to"`
			SMTPAddress      email.SMTPAddress `json:"smtp_address"`
			SMTPHello        string            `json:"smtp_hello"`
			SMTPIdentity     string            `json:"smtp_identity"`
			SMTPUser         string            `json:"smtp_user"`
			SMTPPassword     string            `json:"smtp_password"`
		}{
			From: email.Identity{
				Address: sharedTypes.Email("no-reply@" + emailHost),
			},
			FallbackReplyTo: email.Identity{
				Address: sharedTypes.Email("support@" + emailHost),
			},
			SMTPAddress:  email.SMTPAddress(f.SMTPAddress),
			SMTPHello:    "localhost",
			SMTPUser:     f.SMTPUser,
			SMTPPassword: f.SMTPPassword,
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
		LearnImageCacheBase: sharedTypes.DirName(path.Join(f.TmpDir, "learn-images")),
		ManifestPath:        f.ManifestPath,
		Nav:                 templates.NavOptions{},
		PDFDownloadDomain:   webTypes.PDFDownloadDomain(f.PDFDownloadDomainRaw),
		Sentry:              webTypes.SentryOptions{},
		SiteURL:             siteURL,
		SmokeTest: struct {
			Email     sharedTypes.Email     `json:"email"`
			Password  webTypes.UserPassword `json:"password"`
			ProjectId sharedTypes.UUID      `json:"projectId"`
			UserId    sharedTypes.UUID      `json:"userId"`
		}{
			Email:     sharedTypes.Email(f.SmokeTestUserEmail),
			Password:  webTypes.UserPassword(f.SmokeTestUserPassword),
			ProjectId: sharedTypes.UUID{42},
			UserId:    sharedTypes.UUID{13, 37},
		},
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
			Filestore      objectStorage.Options `json:"filestore"`
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
				URL: clsiURL,
				Persistence: struct {
					CookieName string        `json:"cookie_name"`
					TTL        time.Duration `json:"ttl"`
				}{
					CookieName: f.CLSICookieName,
					TTL:        6 * time.Hour,
				},
			},
			Filestore: f.FilestoreOptions,
			LinkedURLProxy: struct {
				Chain []sharedTypes.URL `json:"chain"`
			}{
				Chain: linkedURLProxyChain,
			},
		},
		JWT: struct {
			Project      jwtOptions.JWTOptions `json:"project"`
			LoggedInUser jwtOptions.JWTOptions `json:"logged_in_user"`
		}{
			Project:      f.JWTOptionsProject,
			LoggedInUser: f.JWTOptionsLoggedInUser,
		},
		SessionCookie: signedCookie.Options{
			Domain:  siteURL.Hostname(),
			Expiry:  7 * 24 * time.Hour,
			Name:    f.SessionCookieName,
			Path:    siteURL.Path,
			Secrets: strings.Split(f.SessionCookieSecretsRaw, ","),
		},
		RateLimits: struct {
			LinkSharingTokenLookupConcurrency int64 `json:"link_sharing_token_lookup_concurrency"`
		}{
			LinkSharingTokenLookupConcurrency: 1,
		},
	}

	policy := fmt.Sprintf(`
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:ListBucket"
      ],
      "Resource": "arn:aws:s3:::%s"
    },
    {
      "Effect": "Allow",
      "Action": [
        "s3:PutObject",
        "s3:GetObject",
        "s3:DeleteObject"
      ],
      "Resource": "arn:aws:s3:::%s/*"
    }
  ]
}
`, f.FilestoreOptions.Bucket, f.FilestoreOptions.Bucket)

	return Config{
		DockerHost:               f.DockerHost(),
		DockerSocket:             f.DockerSocket,
		DockerSocketGroupId:      f.DockerSocketGroupId,
		DockerUser:               f.DockerContainerUser,
		NginxInternalPort:        f.NginxInternalPort,
		NginxPublicListenAddress: f.NginxPublicListenAddress,
		TmpDir:                   f.TmpDir,
		SiteURL:                  siteURL,
		CLSIOptions:              clsiOptions,
		DocumentUpdaterOptions:   documentUpdaterOptions,
		FilestoreOptions:         f.FilestoreOptions,
		LinkedURLProxyToken:      f.LinkedURLProxyToken,
		RealTimeOptions:          realTimeOptions,
		SpellingOptions:          spellingOptions,
		WebOptions:               webOptions,
		CleanupOtherS3Keys:       f.CleanupOtherS3Keys,
		MinioRootUser:            f.MinioRootUser,
		MinioRootPassword:        f.MinioRootPassword,
		S3PolicyContent:          policy,
		S3PolicyName:             f.S3PolicyName,
	}
}

func genSecret(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(errors.Tag(err, "generate secret"))
	}
	return hex.EncodeToString(b)
}

func isSocket(p string) bool {
	p = strings.TrimPrefix(p, "unix://")
	if s, err := os.Stat(p); err != nil {
		return false
	} else if s.Mode()&os.ModeSocket == 0 {
		return false
	}
	return true
}

func mustParseURL(label, s string) sharedTypes.URL {
	u, err := sharedTypes.ParseAndValidateURL(s)
	if err != nil {
		panic(errors.Tag(err, label))
	}
	return *u
}

func mustSetEnv(key, value string) {
	if err := os.Setenv(key, value); err != nil {
		panic(errors.Tag(err, "set "+key))
	}
}

func validateAndSetEnv(name string, d interface{ Validate() error }) {
	if err := d.Validate(); err != nil {
		panic(errors.Tag(err, "validate "+name))
	}
	blob, err := json.Marshal(d)
	if err != nil {
		panic(errors.Tag(err, "serialize "+name))
	}
	mustSetEnv(name, string(blob))
}
