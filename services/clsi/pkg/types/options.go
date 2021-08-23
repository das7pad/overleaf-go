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

	"github.com/das7pad/overleaf-go/pkg/errors"
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

func (o *Options) Validate() error {
	if o.CacheBaseDir == "" {
		return &errors.ValidationError{
			Msg: "missing cache_base_dir",
		}
	}
	if o.CompileBaseDir == "" {
		return &errors.ValidationError{
			Msg: "missing compile_base_dir",
		}
	}
	if o.OutputBaseDir == "" {
		return &errors.ValidationError{
			Msg: "missing output_base_dir",
		}
	}
	if o.ParallelResourceWrite == 0 {
		return &errors.ValidationError{
			Msg: "missing parallel_resource_write",
		}
	}
	if o.MaxFilesAndDirsPerProject == 0 {
		return &errors.ValidationError{
			Msg: "missing max_files_and_dirs_per_project",
		}
	}
	if o.URLDownloadRetries < 0 {
		return &errors.ValidationError{
			Msg: "url_download_retries cannot be negative",
		}
	}
	if o.URLDownloadTimeout < 1 {
		return &errors.ValidationError{
			Msg: "url_download_timeout_ns cannot be lower than 1",
		}
	}
	maxCompileTime := time.Duration(MaxTimeout)
	if o.ProjectCacheDuration < maxCompileTime {
		return &errors.ValidationError{
			Msg: "project_cache_duration_ns cannot be lower than " +
				maxCompileTime.String(),
		}
	}
	if o.GetCapacityRefreshEvery < 1 {
		return &errors.ValidationError{
			Msg: "get_capacity_refresh_every_ns cannot be lower than 1",
		}
	}
	if o.HealthCheckRefreshEvery < 1 {
		return &errors.ValidationError{
			Msg: "health_check_refresh_every_ns cannot be lower than 1",
		}
	}
	return nil
}
