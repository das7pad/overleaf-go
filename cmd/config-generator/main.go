// Golang port of Overleaf
// Copyright (C) 2023 Jakob Ackermann <das7pad@outlook.com>
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
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	configGenerator "github.com/das7pad/overleaf-go/cmd/config-generator/pkg/config-generator"
	"github.com/das7pad/overleaf-go/pkg/email"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/options/jwtOptions"
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

	f := configGenerator.NewFlags()

	flag.BoolVar(&f.DockerComposeSetup, "docker-compose-setup", f.DockerComposeSetup, "generate config for docker-compose setup (hostnames refer to services in docker-compose config)")
	flag.StringVar(&f.MinioRootUser, "minio-root-user", f.MinioRootUser, "minio root access key (default: generated)")
	flag.StringVar(&f.MinioRootPassword, "minio-root-password", f.MinioRootPassword, "minio root secret key, use '-' for prompt (default: generated)")
	flag.StringVar(&f.NginxPublicListenAddress, "nginx-external-listen-address", f.NginxPublicListenAddress, "docker-compose only: public bind address for nginx plain HTTP server")
	flag.StringVar(&f.NginxInternalPort, "nginx-internal-port", f.NginxInternalPort, "docker-compose only: container internal port for nginx plain HTTP server")
	flag.StringVar(&f.TmpDir, "tmp-dir", f.TmpDir, "base dir for ephemeral files")

	flag.StringVar(&f.DockerSocket, "docker-socket", f.DockerSocket, "docker socket path")
	flag.IntVar(&f.DockerSocketGroupId, "docker-socket-group-id", f.DockerSocketGroupId, "docker socket group-id (default: auto-detect from socket)")
	flag.StringVar(&f.DockerContainerUser, "docker-container-user", f.DockerContainerUser, "user inside the docker containers running texlive and services/clsi or cmd/overleaf")
	flag.StringVar(&f.TexLiveImagesRaw, "texlive-images", f.TexLiveImagesRaw, "comma separated list of texlive docker images, first image is default image")

	flag.StringVar(&f.SiteURLRaw, "site-url", f.SiteURLRaw, "site url")
	flag.StringVar(&f.ManifestPath, "frontend-manifest-path", f.ManifestPath, "frontend manifest path, use 'cdn' for download at boot time")
	flag.StringVar(&f.CDNURLRaw, "cdn-url", f.CDNURLRaw, "cdn url")

	flag.StringVar(&f.LinkedURLProxyToken, "linked-url-proxy-token", f.LinkedURLProxyToken, "proxy token (local instance)")
	flag.StringVar(&f.LinkedURLProxyChainRaw, "linked-url-proxy-chain", f.LinkedURLProxyChainRaw, "proxy chain (comma separated list, first item is first proxy hop)")

	flag.StringVar(&f.CLSIURLRaw, "clsi-url", f.CLSIURLRaw, "clsi url (required when load balancing compiles)")
	flag.StringVar(&f.CLSICookieName, "clsi-cookie-name", f.CLSICookieName, "clsi cookie name (required when load balancing compiles)")
	flag.StringVar(&f.PDFDownloadDomainRaw, "pdf-download-url", f.PDFDownloadDomainRaw, "pdf download url")

	flag.StringVar(&f.SessionCookieName, "session-cookie-name", f.SessionCookieName, "session cookie name")
	flag.StringVar(&f.SessionCookieSecretsRaw, "session-cookie-secrets", f.SessionCookieSecretsRaw, "session cookie secrets (comma separated list, first item is used for new cookies)")

	flag.StringVar(&f.SmokeTestUserEmail, "smoke-test-user-email", f.SmokeTestUserEmail, "(default: smoke-test@<site-url-domain>)")
	flag.StringVar(&f.SmokeTestUserPassword, "smoke-test-user-password", f.SmokeTestUserPassword, "(default: generated")

	flag.StringVar(&f.SMTPAddress, "email-smtp-address", f.SMTPAddress, "address:port of email provider")
	flag.StringVar(&f.SMTPUser, "email-smtp-user", f.SMTPUser, "login user name at email provider")
	flag.StringVar(&f.SMTPPassword, "email-smtp-password", f.SMTPPassword, "login password at email provider, use '-' for prompt")

	flag.StringVar(&f.FilestoreOptions.Bucket, "filestore-bucket", f.FilestoreOptions.Bucket, "bucket for binary files")
	flag.StringVar(&f.FilestoreOptions.Endpoint, "s3-endpoint", f.FilestoreOptions.Endpoint, "endpoint of s3 compatible storage backend (e.g. minio)")
	flag.BoolVar(&f.FilestoreOptions.Secure, "s3-https", f.FilestoreOptions.Secure, "toggle to use https on s3-endpoint")
	flag.StringVar(&f.FilestoreOptions.Region, "s3-region", f.FilestoreOptions.Region, "region of s3 bucket")
	flag.StringVar(&f.FilestoreOptions.Key, "s3-key", f.FilestoreOptions.Key, "s3 access key (default: generate)")
	flag.StringVar(&f.FilestoreOptions.Secret, "s3-secret", f.FilestoreOptions.Secret, "s3 secret key, use '-' for prompt (default: generated)")

	flagJWTOptions(&f.JWTOptionsLoggedInUser, "jwt-logged-in-user")
	flagJWTOptions(&f.JWTOptionsProject, "jwt-project")

	flag.Parse()
	if !email.SMTPAddress(f.SMTPAddress).IsSpecial() {
		handlePromptInput(&f.SMTPPassword, "SMTP Password")
	}
	handlePromptInput(&f.FilestoreOptions.Secret, "S3 secret key")
	handlePromptInput(&f.MinioRootPassword, "Minio root user password")
	handlePromptInput(&f.JWTOptionsLoggedInUser.Key, "JWT logged in user key")
	handlePromptInput(&f.JWTOptionsProject.Key, "JWT project key")
	handlePromptInput(&f.SmokeTestUserPassword, "Smoke test user password")

	fmt.Println("# Generated using")
	fmt.Printf("# $ %s", os.Args[0])
	flag.VisitAll(func(f *flag.Flag) {
		fmt.Printf(" --%s=%s", f.Name, f.Value)
	})
	fmt.Println()
	fmt.Println()

	c := configGenerator.Generate(f)

	fmt.Println("# docker")
	fmt.Printf("DOCKER_SOCKET=%s\n", c.DockerSocket)
	fmt.Printf("DOCKER_SOCKET_GROUP_ID=%d\n", c.DockerSocketGroupId)
	fmt.Printf("DOCKER_USER=%s\n", c.CLSIOptions.DockerContainerOptions.User)
	fmt.Printf("NGINX_PUBLIC_LISTEN_ADDRESS=%s\n", c.NginxPublicListenAddress)
	fmt.Printf("NGINX_INTERNAL_PORT=%s\n", c.NginxInternalPort)
	fmt.Printf("TMP_DIR=%s\n", c.TmpDir)

	fmt.Println("# services/spelling or services/web or cmd/overleaf:")
	fmt.Printf("PUBLIC_URL=%s\n", c.SiteURL.String())
	fmt.Println()

	fmt.Println("# services/clsi or cmd/overleaf:")
	fmt.Printf("CLSI_OPTIONS=%s\n", serialize(&c.CLSIOptions, "clsi options"))
	fmt.Printf("DOCKER_HOST=unix://%s\n", c.DockerSocket)
	fmt.Println()

	fmt.Println("# services/document-updater or cmd/overleaf:")
	fmt.Printf("DOCUMENT_UPDATER_OPTIONS=%s\n", serialize(&c.DocumentUpdaterOptions, "document updater options"))
	fmt.Println()

	fmt.Println("# services/linked-url-proxy:")
	fmt.Printf("PROXY_TOKEN=%s\n", c.LinkedURLProxyToken)
	fmt.Println()

	fmt.Println("# services/real-time or cmd/overleaf:")
	fmt.Printf("REAL_TIME_OPTIONS=%s\n", serialize(&c.RealTimeOptions, "realtime options"))
	fmt.Println()

	fmt.Println("# services/spelling or cmd/overleaf:")
	fmt.Printf("SPELLING_OPTIONS=%s\n", serialize(&c.SpellingOptions, "spelling options"))
	fmt.Println()

	fmt.Println("# services/web or cmd/overleaf:")
	fmt.Printf("WEB_OPTIONS=%s\n", serialize(&c.WebOptions, "web options"))
	fmt.Println()

	fmt.Println("# s3:")
	fmt.Printf("BUCKET=%s\n", c.FilestoreOptions.Bucket)
	fmt.Printf("ACCESS_KEY=%s\n", c.FilestoreOptions.Key)
	fmt.Printf("SECRET_KEY=%s\n", c.FilestoreOptions.Secret)
	fmt.Println()
	fmt.Println("# s3 alternative 'minio':")
	fmt.Printf("# Run minio on MINIO_ENDPOINT\n")
	fmt.Printf("MINIO_ENDPOINT=%s\n", c.FilestoreOptions.Endpoint)
	fmt.Printf("MINIO_SECURE=%t\n", c.FilestoreOptions.Secure)
	fmt.Printf("MINIO_ROOT_USER=%s\n", c.MinioRootUser)
	fmt.Printf("MINIO_ROOT_PASSWORD=%s\n", c.MinioRootPassword)
	fmt.Printf("MINIO_REGION=%s\n", c.FilestoreOptions.Region)
	fmt.Println()
	fmt.Println("# s3 and minio:")
	fmt.Println("# Setup an privileged user using the ACCESS_KEY/SECRET_KEY and restrict access with a policy.")
	fmt.Println("# Below is a minimal policy that allows delete/listing/read/write access:")
	fmt.Printf("S3_POLICY_CONTENT=%s\n", strings.Join(strings.Fields(c.S3PolicyContent), ""))
	fmt.Printf("S3_POLICY_NAME=%s\n", c.S3PolicyName)
	fmt.Println()
	fmt.Println("# Cleanup prior minio users, e.g. after cycling s3 credentials")
	fmt.Printf("CLEANUP_OTHER_S3_KEYS=%t\n", c.CleanupOtherS3Keys)
	fmt.Println()
}

func flagJWTOptions(o *jwtOptions.JWTOptions, prefix string) {
	flag.StringVar(&o.Algorithm, prefix+"-algo", o.Algorithm, "algorithm")
	flag.StringVar(&o.Key, prefix+"-key", o.Key, "key (default: generated)")
	flag.DurationVar(&o.ExpiresIn, prefix+"-expiry", o.ExpiresIn, "expiry")
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
