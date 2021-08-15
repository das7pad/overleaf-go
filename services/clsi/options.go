// Golang port of the Overleaf clsi service
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

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/das7pad/clsi/pkg/errors"
	"github.com/das7pad/clsi/pkg/types"
)

func getIntFromEnv(key string, fallback int64) int64 {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		panic(err)
	}
	return parsed
}

func getStringFromEnv(key, fallback string) string {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	return raw
}

func getJSONFromEnv(key string, target interface{}) {
	if v, exists := os.LookupEnv(key); !exists || v == "" {
		panic(errors.New("missing " + key))
	}
	err := json.Unmarshal([]byte(os.Getenv(key)), target)
	if err != nil {
		panic(fmt.Errorf("malformed %s: %w", key, err))
	}
}

type clsiOptions struct {
	address     string
	loadAddress string

	copyExecAgent    bool
	copyExecAgentSrc string
	copyExecAgentDst string

	options *types.Options
}

func getOptions() *clsiOptions {
	o := &clsiOptions{}
	listenAddress := getStringFromEnv("LISTEN_ADDRESS", "localhost")
	port := getIntFromEnv("PORT", 3013)
	o.address = listenAddress + ":" + strconv.FormatInt(port, 10)

	loadPort := getIntFromEnv("LOAD_PORT", 3048)
	o.loadAddress = listenAddress + ":" + strconv.FormatInt(loadPort, 10)

	o.copyExecAgentSrc = getStringFromEnv("COPY_EXEC_AGENT_SRC", "")
	o.copyExecAgentDst = getStringFromEnv("COPY_EXEC_AGENT_DST", "")
	o.copyExecAgent = getStringFromEnv("COPY_EXEC_AGENT", "false") == "true"

	getJSONFromEnv("OPTIONS", &o.options)

	if o.options.CacheBaseDir == "" {
		panic("missing cache_base_dir")
	}
	if o.options.CompileBaseDir == "" {
		panic("missing compile_base_dir")
	}
	if o.options.OutputBaseDir == "" {
		panic("missing output_base_dir")
	}
	if o.options.ParallelResourceWrite == 0 {
		panic("missing parallel_resource_write")
	}
	if o.options.MaxFilesAndDirsPerProject == 0 {
		panic("missing max_files_and_dirs_per_project")
	}
	if o.options.URLDownloadRetries < 0 {
		panic("url_download_retries cannot be negative")
	}
	if o.options.URLDownloadTimeout < 1 {
		panic("url_download_timeout_ns cannot be lower than 1")
	}
	maxCompileTime := time.Duration(types.MaxTimeout)
	if o.options.ProjectCacheDuration < maxCompileTime {
		panic(
			"project_cache_duration_ns cannot be lower than " +
				maxCompileTime.String(),
		)
	}
	if o.options.GetCapacityRefreshEvery < 1 {
		panic("get_capacity_refresh_every_ns cannot be lower than 1")
	}
	if o.options.HealthCheckRefreshEvery < 1 {
		panic("health_check_refresh_every_ns cannot be lower than 1")
	}

	if o.options.DockerContainerOptions.SeccompPolicyPath == "" {
		o.options.DockerContainerOptions.SeccompPolicyPath = getStringFromEnv(
			"SECCOMP_POLICY_PATH",
			"-",
		)
	}

	return o
}
