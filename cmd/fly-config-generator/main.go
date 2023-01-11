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
	"errors"
	"flag"
	"fmt"
	"strings"
	"text/template"
)

func main() {
	d := struct {
		App string
	}{}
	flag.StringVar(&d.App, "app", "", "unique app name")
	flag.Parse()

	if d.App == "" {
		panic(errors.New("--app is a required argument"))
	}

	t := template.Must(template.New("").Parse(strings.TrimLeft(`
app = {{ printf "%q" .App }}
kill_signal = "SIGINT"
# LINKED_URL_PROXY_TIMEOUT_MS in seconds plus some 1s extra delay
kill_timeout = 28

[build]
  builder = "paketobuildpacks/builder:base"
  buildpacks = ["gcr.io/paketo-buildpacks/go"]
[build.args]
  BP_GO_TARGETS = "./services/linked-url-proxy"

[env]
  ALLOW_REDIRECTS = "true"
  LINKED_URL_PROXY_TIMEOUT_MS = "27000"
  LISTEN_ADDRESS = "0.0.0.0"
  PORT = "8080"

[[services]]
  internal_port = 8080
  processes = ["app"]
  protocol = "tcp"
  script_checks = []
  [services.concurrency]
    hard_limit = 200
    soft_limit = 100
    type = "connections"

  [[services.http_checks]]
    grace_period = "1s"
    interval = "5s"
    method = "get"
    path = "/status"
    protocol = "http"
    restart_limit = 3
    timeout = "5s"

  [[services.ports]]
    handlers = ["tls", "http"]
    port = 443
`, "\n")))

	b := strings.Builder{}
	if err := t.Execute(&b, d); err != nil {
		panic(err)
	}
	fmt.Println(b.String())
}
