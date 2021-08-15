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

package types

import (
	"time"
)

type SeccompPolicy struct {
	DefaultAction string   `json:"defaultAction"`
	Architectures []string `json:"architectures"`
	Syscalls      []struct {
		Name   string `json:"name"`
		Action string `json:"action"`
		Args   []struct {
			Index    int    `json:"index"`
			Value    int    `json:"value"`
			ValueTwo int    `json:"valueTwo"`
			Op       string `json:"op"`
		} `json:"args"`
	} `json:"syscalls"`
}

type DockerContainerOptions struct {
	User                   string        `json:"user"`
	Env                    Environment   `json:"env"`
	AgentPathContainer     string        `json:"agent_path_container"`
	AgentPathHost          string        `json:"agent_path_host"`
	AgentContainerLifeSpan time.Duration `json:"agent_container_life_span_ns"`
	AgentRestartAttempts   int64         `json:"agent_restart_attempts"`

	Runtime           string `json:"runtime"`
	SeccompPolicyPath string `json:"seccomp_policy_path"`

	CompileBaseDir CompileDirBase `json:"compile_base_dir"`
	OutputBaseDir  OutputBaseDir  `json:"output_base_dir"`
}

type Options struct {
	DefaultCompileGroup  CompileGroup   `json:"default_compile_group"`
	DefaultImage         ImageName      `json:"default_image"`
	AllowedImages        []ImageName    `json:"allowed_images"`
	AllowedCompileGroups []CompileGroup `json:"allowed_compile_groups"`

	ProjectCacheDuration    time.Duration `json:"project_cache_duration_ns"`
	GetCapacityRefreshEvery time.Duration `json:"get_capacity_refresh_every_ns"`
	HealthCheckRefreshEvery time.Duration `json:"health_check_refresh_every_ns"`

	ParallelResourceWrite     int64 `json:"parallel_resource_write"`
	MaxFilesAndDirsPerProject int64 `json:"max_files_and_dirs_per_project"`

	URLDownloadRetries int64         `json:"url_download_retries"`
	URLDownloadTimeout time.Duration `json:"url_download_timeout_ns"`

	CacheBaseDir   CacheBaseDir   `json:"cache_base_dir"`
	CompileBaseDir CompileDirBase `json:"compile_base_dir"`
	OutputBaseDir  OutputBaseDir  `json:"output_base_dir"`

	LatexBaseEnv Environment `json:"latex_base_env"`

	Runner                 string `json:"runner"`
	DockerContainerOptions `json:"docker_container_options"`
}
