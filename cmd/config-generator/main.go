// Golang port of Overleaf
// Copyright (C) 2022-2023 Jakob Ackermann <das7pad@outlook.com>
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
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
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

func main() {
	defer func() {
		err := recover()
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "Error:", err)
			flag.CommandLine.Usage()
			os.Exit(101)
		}
	}()
	jwtOptionsLoggedInUser := jwtOptions.JWTOptions{
		Algorithm: "HS512",
		Key:       genSecret(32),
		ExpiresIn: 24 * time.Hour,
	}
	jwtOptionsProject := jwtOptions.JWTOptions{
		Algorithm: "HS512",
		Key:       genSecret(32),
		ExpiresIn: time.Hour,
	}
	linkedURLProxyToken := genSecret(32)

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

	dockerComposeSetup := true
	flag.BoolVar(&dockerComposeSetup, "docker-compose-setup", dockerComposeSetup, "generate config for docker-compose setup (hostnames refer to services in docker-compose config)")

	nginxPublicListenAddress := "127.0.0.1:8080"
	flag.StringVar(&nginxPublicListenAddress, "nginx-external-listen-address", nginxPublicListenAddress, "docker-compose only: public bind address for nginx plain HTTP server")
	nginxInternalPort := "8080"
	flag.StringVar(&nginxInternalPort, "nginx-internal-port", nginxInternalPort, "docker-compose only: container internal port for nginx plain HTTP server")

	flag.StringVar(&dockerSocket, "docker-socket", dockerSocket, "docker socket path")
	dockerSocketGroupId := -1
	flag.IntVar(&dockerSocketGroupId, "docker-socket-group-id", dockerSocketGroupId, "docker socket group-id (default: auto-detect from socket)")
	dockerContainerUser := "nobody"
	flag.StringVar(&dockerContainerUser, "docker-container-user", dockerContainerUser, "user inside the docker containers running texlive and services/clsi or cmd/overleaf")

	texLiveImages := "texlive/texlive:TL2021-historic"
	flag.StringVar(&texLiveImages, "texlive-images", texLiveImages, "comma separated list of texlive docker images, first image is default image")

	tmpDir := "/tmp/ol"
	flag.StringVar(&tmpDir, "tmp-dir", tmpDir, "base dir for ephemeral files")

	manifestPath := ""
	flag.StringVar(&manifestPath, "frontend-manifest-path", manifestPath, "frontend manifest path, use 'cdn' for download at boot time")

	siteURLRaw := "http://localhost:8080"
	flag.StringVar(&siteURLRaw, "site-url", siteURLRaw, "site url")

	cdnURLRaw := ""
	flag.StringVar(&cdnURLRaw, "cdn-url", cdnURLRaw, "cdn url")

	linkedURLProxyChainRaw := ""
	flag.StringVar(&linkedURLProxyChainRaw, "linked-url-proxy-chain", linkedURLProxyChainRaw, "proxy chain (comma separated list, first item is first proxy hop)")

	clsiURLRaw := "http://127.0.0.1:3013"
	flag.StringVar(&clsiURLRaw, "clsi-url", clsiURLRaw, "clsi url (required when load balancing compiles)")

	pdfDownloadURLRaw := ""
	flag.StringVar(&pdfDownloadURLRaw, "pdf-download-url", pdfDownloadURLRaw, "pdf download url")

	clsiCookieName := ""
	flag.StringVar(&clsiCookieName, "clsi-cookie-name", clsiCookieName, "clsi cookie name (required when load balancing compiles)")

	sessionCookieName := "ol.go"
	flag.StringVar(&sessionCookieName, "session-cookie-name", sessionCookieName, "session cookie name")

	smtpAddress := "log"
	flag.StringVar(&smtpAddress, "email-smtp-address", smtpAddress, "address:port of email provider")
	smtpUser := ""
	flag.StringVar(&smtpUser, "email-smtp-user", smtpUser, "login user name at email provider")
	smtpPassword := "-"
	flag.StringVar(&smtpPassword, "email-smtp-password", smtpPassword, "login password at email provider, use '-' for prompt")

	filestoreOptions := objectStorage.Options{
		Bucket:          "overleaf-files",
		Provider:        "minio",
		Endpoint:        "127.0.0.1:9000",
		Region:          "us-east-1",
		Secure:          false,
		Key:             "",
		Secret:          "",
		SignedURLExpiry: 15 * time.Minute,
	}
	flag.StringVar(&filestoreOptions.Bucket, "filestore-bucket", filestoreOptions.Bucket, "bucket for binary files")
	flag.StringVar(&filestoreOptions.Endpoint, "s3-endpoint", filestoreOptions.Endpoint, "endpoint of s3 compatible storage backend (e.g. minio)")
	flag.BoolVar(&filestoreOptions.Secure, "s3-https", filestoreOptions.Secure, "toggle to use https on s3-endpoint")
	flag.StringVar(&filestoreOptions.Region, "s3-region", filestoreOptions.Region, "region of s3 bucket")
	flag.StringVar(&filestoreOptions.Key, "s3-key", filestoreOptions.Key, "s3 access key (default: generate)")
	flag.StringVar(&filestoreOptions.Secret, "s3-secret", filestoreOptions.Secret, "s3 secret key, use '-' for prompt (default: generate)")

	flag.Parse()

	if !email.SMTPAddress(smtpAddress).IsSpecial() {
		handlePromptInput(&smtpPassword, "SMTP Password")
	}
	handlePromptInput(&filestoreOptions.Secret, "S3 secret key")

	if filestoreOptions.Key == "" {
		filestoreOptions.Key = genSecret(32)
	}
	if filestoreOptions.Secret == "" {
		filestoreOptions.Secret = genSecret(32)
	}

	siteURL := mustParseURL("site-url", siteURLRaw)

	if cdnURLRaw == "" {
		cdnURLRaw = siteURL.JoinPath("/assets/").String()
	}
	cdnURL := mustParseURL("cdn-url", cdnURLRaw)

	var clsiURL sharedTypes.URL
	if clsiURLRaw != "" {
		clsiURL = mustParseURL("clsi-url", clsiURLRaw)
	}

	if dockerComposeSetup && linkedURLProxyChainRaw == "" {
		linkedURLProxyChainRaw = "http://linked-url-proxy:8080/proxy/" + linkedURLProxyToken
	}
	linkedURLProxyChain := make([]sharedTypes.URL, 0)
	for i, s := range strings.Split(linkedURLProxyChainRaw, ",") {
		s = strings.Trim(s, `"' `)
		u := mustParseURL(fmt.Sprintf("linked-url-proxy-chain idx=%d", i), s)
		linkedURLProxyChain = append(linkedURLProxyChain, u)
	}

	rawImages := strings.Split(texLiveImages, ",")
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
	agentPathHost := path.Join(tmpDir, "exec-agent")

	if manifestPath == "" {
		manifestPath = path.Join(tmpDir, "assets/manifest.json")
	}

	if dockerComposeSetup && filestoreOptions.Endpoint == "127.0.0.1:9000" {
		filestoreOptions.Endpoint = "minio:9000"
	}

	if dockerSocketGroupId == -1 {
		s := syscall.Stat_t{}
		if err := syscall.Stat(dockerSocketRootful, &s); err != nil {
			panic(errors.Tag(err, "detect docker group-id on docker socket"))
		}
		dockerSocketGroupId = int(s.Gid)
	}

	fmt.Println("# docker")
	fmt.Printf("DOCKER_SOCKET=%s\n", dockerSocket)
	fmt.Printf("DOCKER_SOCKET_GROUP_ID=%d\n", dockerSocketGroupId)
	fmt.Printf("DOCKER_USER=%s\n", dockerContainerUser)
	fmt.Printf("NGINX_PUBLIC_LISTEN_ADDRESS=%s\n", nginxPublicListenAddress)
	fmt.Printf("NGINX_INTERNAL_PORT=%s\n", nginxInternalPort)
	fmt.Printf("TMP_DIR=%s\n", tmpDir)

	fmt.Println("# services/spelling or services/web or cmd/overleaf:")
	fmt.Printf("PUBLIC_URL=%s\n", siteURL.String())
	fmt.Println()

	fmt.Println("# services/linked-url-proxy:")
	fmt.Printf("PROXY_TOKEN=%s\n", linkedURLProxyToken)
	fmt.Println()

	clsiOptions := clsiTypes.Options{
		AllowedImages:             allowedImages,
		CopyExecAgentDst:          agentPathHost,
		ProjectCacheDuration:      time.Hour,
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
	fmt.Println("# services/clsi or cmd/overleaf:")
	fmt.Printf("CLSI_OPTIONS=%s\n", serialize(&clsiOptions, "clsi options"))
	fmt.Printf("DOCKER_HOST=unix://%s\n", dockerSocket)
	fmt.Println()

	documentUpdaterOptions := documentUpdaterTypes.Options{
		Workers:                      20,
		PendingUpdatesListShardCount: 1,
	}
	fmt.Println("# services/document-updater or cmd/overleaf:")
	fmt.Printf("DOCUMENT_UPDATER_OPTIONS=%s\n", serialize(&documentUpdaterOptions, "document updater options"))
	fmt.Println()

	realTimeOptions := realTimeTypes.Options{
		GracefulShutdown: struct {
			Delay   time.Duration `json:"delay"`
			Timeout time.Duration `json:"timeout"`
		}{
			Delay:   3 * time.Second,
			Timeout: 10 * time.Second,
		},
		JWT: struct {
			Project jwtOptions.JWTOptions `json:"project"`
		}{
			Project: jwtOptionsProject,
		},
	}
	fmt.Println("# services/real-time or cmd/overleaf:")
	fmt.Printf("REAL_TIME_OPTIONS=%s\n", serialize(&realTimeOptions, "realtime options"))
	fmt.Println()

	spellingOptions := spellingTypes.Options{
		LRUSize: 10_000,
	}
	fmt.Println("# services/spelling or cmd/overleaf:")
	fmt.Printf("SPELLING_OPTIONS=%s\n", serialize(&spellingOptions, "spelling options"))
	fmt.Println()

	emailHost := siteURL.Hostname()
	webOptions := webTypes.Options{
		AdminEmail:        sharedTypes.Email("support@" + emailHost),
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
			SMTPAddress:  email.SMTPAddress(smtpAddress),
			SMTPHello:    "localhost",
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
			Email:     sharedTypes.Email("smoke-test@" + emailHost),
			Password:  webTypes.UserPassword(genSecret(72 / 2)),
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
					CookieName: clsiCookieName,
					TTL:        6 * time.Hour,
				},
			},
			Filestore: filestoreOptions,
			LinkedURLProxy: struct {
				Chain []sharedTypes.URL `json:"chain"`
			}{
				Chain: linkedURLProxyChain,
			},
		},
		JWT: struct {
			Compile      jwtOptions.JWTOptions `json:"compile"`
			LoggedInUser jwtOptions.JWTOptions `json:"logged_in_user"`
		}{
			Compile:      jwtOptionsProject,
			LoggedInUser: jwtOptionsLoggedInUser,
		},
		SessionCookie: signedCookie.Options{
			Domain:  siteURL.Hostname(),
			Expiry:  7 * 24 * time.Hour,
			Name:    sessionCookieName,
			Path:    siteURL.Path,
			Secrets: []string{genSecret(32)},
		},
		RateLimits: struct {
			LinkSharingTokenLookupConcurrency int64 `json:"link_sharing_token_lookup_concurrency"`
		}{
			LinkSharingTokenLookupConcurrency: 1,
		},
	}

	fmt.Println("# services/web or cmd/overleaf:")
	fmt.Printf("WEB_OPTIONS=%s\n", serialize(&webOptions, "web options"))
	fmt.Println()

	fmt.Println("# s3:")
	fmt.Printf("BUCKET=%s\n", filestoreOptions.Bucket)
	fmt.Printf("ACCESS_KEY=%s\n", filestoreOptions.Key)
	fmt.Printf("SECRET_KEY=%s\n", filestoreOptions.Secret)
	fmt.Println()
	minioRootUser := genSecret(32)
	minioRootPassword := genSecret(32)
	fmt.Println("# s3 alternative 'minio':")
	fmt.Printf("# Run minio on MINIO_ENDPOINT\n")
	fmt.Printf("MINIO_ENDPOINT=%s\n", filestoreOptions.Endpoint)
	fmt.Printf("MINIO_SECURE=%t\n", filestoreOptions.Secure)
	fmt.Printf("MINIO_ROOT_USER=%s\n", minioRootUser)
	fmt.Printf("MINIO_ROOT_PASSWORD=%s\n", minioRootPassword)
	fmt.Printf("MINIO_REGION=%s\n", filestoreOptions.Region)
	fmt.Println()
	fmt.Println("# s3 and minio:")
	fmt.Println("# Setup an privileged user using the ACCESS_KEY/SECRET_KEY and restrict access with a policy.")
	fmt.Println("# Below is a minimal policy that allows delete/listing/read/write access:")
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
`, filestoreOptions.Bucket, filestoreOptions.Bucket)
	fmt.Printf("S3_POLICY=%s", strings.Join(strings.Fields(policy), ""))
	fmt.Println()
}

func genSecret(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(errors.Tag(err, "generate secret"))
	}
	return hex.EncodeToString(b)
}

func handlePromptInput(dst *string, label string) {
	if *dst != "-" {
		return
	}
	_, _ = fmt.Fprintf(
		os.Stderr,
		"Please type the %s and confirm with ENTER: ",
		label,
	)
	s := bufio.NewScanner(os.Stdin)
	if !s.Scan() {
		fmt.Println()
		panic(errors.Tag(s.Err(), "read "+label))
	}
	*dst = s.Text()
	fmt.Println()
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

func serialize(d interface{ Validate() error }, label string) string {
	if err := d.Validate(); err != nil {
		panic(errors.Tag(err, "validate "+label))
	}
	blob, err := json.Marshal(d)
	if err != nil {
		panic(errors.Tag(err, "serialize "+label))
	}
	return string(blob)
}
