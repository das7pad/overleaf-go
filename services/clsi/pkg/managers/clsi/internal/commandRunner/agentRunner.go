// Golang port of Overleaf
// Copyright (C) 2021-2023 Jakob Ackermann <das7pad@outlook.com>
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

package commandRunner

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	dockerTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/go-units"

	"github.com/das7pad/overleaf-go/pkg/copyFile"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/constants"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/commandRunner/internal/seccomp"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type agentRunner struct {
	dockerClient *client.Client
	d            *net.Dialer

	allowedImages           []sharedTypes.ImageName
	compileBaseDir          types.CompileBaseDir
	o                       types.DockerContainerOptions
	seccompPolicy           string
	currentClsiProcessEpoch string
	tries                   int64
}

func containerName(namespace types.Namespace) string {
	return "project-" + string(namespace)
}

func copyAgent(dst, src string) error {
	if src == "" || src == "-" || dst == "" || dst == "-" {
		// copying of the agent is not configured or explicitly disabled
		return nil
	}
	agent, err := os.Open(src)
	if err != nil {
		return errors.Tag(err, "open agent for copying")
	}
	defer func() {
		_ = agent.Close()
	}()
	if err = copyFile.AtomicWithMode(dst, agent); err != nil {
		return errors.Tag(err, "copy agent")
	}
	return nil
}

func newAgentRunner(options *types.Options) (Runner, error) {
	dockerClient, dockerErr := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if dockerErr != nil {
		return nil, dockerErr
	}

	o := options.DockerContainerOptions
	{
		err := copyAgent(options.CopyExecAgentDst, options.CopyExecAgentSrc)
		if err != nil {
			return nil, err
		}
	}

	runner := agentRunner{
		dockerClient:            dockerClient,
		d:                       &net.Dialer{},
		allowedImages:           options.AllowedImages,
		compileBaseDir:          options.CompileBaseDir,
		tries:                   1 + o.AgentRestartAttempts,
		currentClsiProcessEpoch: time.Now().UTC().Format(time.RFC3339Nano),
	}

	if o.AgentPathContainer == "" {
		o.AgentPathContainer = defaultAgentPathContainer
	}
	if o.AgentContainerLifeSpan == 0 {
		o.AgentContainerLifeSpan = defaultAgentContainerLifeSpan
	}
	if o.CompileBaseDir == "" {
		o.CompileBaseDir = options.CompileBaseDir
	}
	if o.OutputBaseDir == "" {
		o.OutputBaseDir = options.OutputBaseDir
	}

	if o.Env == nil {
		o.Env = make(types.Environment, 0)
	}

	if o.SeccompPolicyPath != "-" {
		policy, err := seccomp.Load(o.SeccompPolicyPath)
		if err != nil {
			return nil, errors.Tag(err, "seccomp policy invalid")
		}
		runner.seccompPolicy = policy
	}

	runner.o = o

	return &runner, nil
}

const (
	defaultAgentContainerLifeSpan = 15 * time.Minute
	defaultAgentPathContainer     = "/opt/exec-agent"
	memoryLimitInBytes            = 1024 * 1024 * 1024 * 1024
	clsiProcessEpochLabel         = "com.overleaf.clsi.process.epoch"
)

func (a *agentRunner) Stop(namespace types.Namespace) error {
	return a.stopContainer(namespace)
}

func (a *agentRunner) Setup(ctx context.Context, namespace types.Namespace, imageName sharedTypes.ImageName) (*time.Time, error) {
	validUntil, err := a.createContainer(ctx, namespace, imageName)
	switch {
	case err == nil:
		// Happy path.
	case errdefs.IsConflict(err):
		// Handle conflict error.
		epoch, _ := a.getContainerEpoch(ctx, namespace)
		if epoch != a.currentClsiProcessEpoch {
			// The container is from previous version/cycle, replace it.
			// - version: options may have changed.
			// - cycle: we lost track of expected/max container life-time.
			err = a.stopContainer(namespace)
			if err != nil {
				return nil, errors.Tag(err, "cannot stop old container")
			}
			// The container is not gone immediately. Delay and retry 3 times.
			for i := 1; i < 4; i++ {
				time.Sleep(time.Duration(i * 100 * int(time.Millisecond)))

				validUntil, err = a.createContainer(ctx, namespace, imageName)
				if err == nil || errdefs.IsConflict(err) {
					break
				}
			}
			if err != nil {
				return nil, errors.Tag(err, "cannot re-create old container")
			}
		} else {
			// The container is running, but expired. Reset it.
			validUntil, err = a.restartContainer(ctx, namespace)
			switch {
			case err == nil:
				// Happy path
			case errdefs.IsNotFound(err):
				// The container just died. Recreate it.
				validUntil, err = a.createContainer(ctx, namespace, imageName)
				if err != nil {
					return nil, errors.Tag(
						err, "cannot re-create expired container",
					)
				}
			default:
				return nil, errors.Tag(
					err, "cannot restart expired container",
				)
			}
		}
	default:
		// Bail out on low-level errors.
		return nil, errors.Tag(err, "low level error while creating container")
	}

	var probeErr error
	// Wait for the startup of the agent.
	for i := 0; i < 5; i++ {
		// Backoff momentarily starting from attempt two.
		time.Sleep(time.Duration(i * 100 * int(time.Millisecond)))

		if probeErr = a.probe(ctx, namespace); probeErr == nil {
			return validUntil, nil
		}
	}
	return nil, probeErr
}

func (a *agentRunner) createContainer(ctx context.Context, namespace types.Namespace, imageName sharedTypes.ImageName) (*time.Time, error) {
	compileDir := a.o.CompileBaseDir.CompileDir(namespace)
	outputDir := a.o.OutputBaseDir.OutputDir(namespace)

	lifeSpanInSeconds := int64(a.o.AgentContainerLifeSpan) / int64(time.Second)

	name := containerName(namespace)

	mounts := []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: string(compileDir),
			Target: constants.CompileDirContainer,
		},
		{
			Type:     mount.TypeBind,
			Source:   string(outputDir),
			Target:   constants.OutputDirContainer,
			ReadOnly: true,
		},
		{
			Type:     mount.TypeBind,
			Source:   a.o.AgentPathHost,
			Target:   a.o.AgentPathContainer,
			ReadOnly: true,
		},
	}

	hostConfig := container.HostConfig{
		LogConfig:   container.LogConfig{Type: "none"},
		NetworkMode: "none",
		AutoRemove:  true,
		CapDrop:     []string{"ALL"},
		SecurityOpt: []string{"no-new-privileges"},
		Resources: container.Resources{
			Memory: memoryLimitInBytes,
			Ulimits: []*units.Ulimit{
				{
					Name: "cpu",
					Soft: lifeSpanInSeconds,
					Hard: lifeSpanInSeconds,
				},
			},
		},
		Runtime: a.o.Runtime,
		Mounts:  mounts,
	}

	if a.seccompPolicy != "" {
		hostConfig.SecurityOpt = append(
			hostConfig.SecurityOpt,
			"seccomp="+a.seccompPolicy,
		)
	}

	env := a.o.Env

	if err := imageName.CheckIsAllowed(a.allowedImages); err != nil {
		return nil, err
	}
	year := imageName.Year()
	PATH := fmt.Sprintf(
		"/usr/local/texlive/%s/bin/x86_64-linux:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		year,
	)
	env = append(env, "PATH="+PATH, "HOME=/tmp")

	_, err := a.dockerClient.ContainerCreate(
		ctx,
		&container.Config{
			Cmd: []string{
				"unix://" + constants.AgentSocketPathContainer,
			},
			Entrypoint:      []string{a.o.AgentPathContainer},
			Env:             env,
			Hostname:        "overleaf-golang-port",
			Image:           string(imageName),
			NetworkDisabled: true,
			User:            a.o.User,
			WorkingDir:      constants.CompileDirContainer,
			Labels: map[string]string{
				clsiProcessEpochLabel: a.currentClsiProcessEpoch,
			},
		},
		&hostConfig,
		nil,
		nil,
		name,
	)
	if err != nil {
		return nil, errors.Tag(err, "cannot create container")
	}

	validUntil := time.Now().Add(a.o.AgentContainerLifeSpan)
	// The container was just created, start it.
	err = a.dockerClient.ContainerStart(
		ctx,
		name,
		dockerTypes.ContainerStartOptions{},
	)
	if err != nil {
		return nil, errors.Tag(err, "cannot start container")
	}
	return &validUntil, nil
}

func (a *agentRunner) getContainerEpoch(ctx context.Context, namespace types.Namespace) (string, error) {
	res, err := a.dockerClient.ContainerInspect(ctx, containerName(namespace))
	if err != nil {
		return "", errors.Tag(err, "cannot get container epoch")
	}
	if res.Config == nil {
		return "", errors.New("container config missing")
	}
	return res.Config.Labels[clsiProcessEpochLabel], nil
}

func (a *agentRunner) probe(ctx context.Context, namespace types.Namespace) error {
	timeout := 4242 * time.Millisecond
	options := &types.CommandOptions{
		CommandLine:        types.CommandLine{"true"},
		CommandOutputFiles: types.CommandOutputFiles{},
		Environment:        types.Environment{},
		ComputeTimeout:     sharedTypes.ComputeTimeout(timeout),
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	code, err := a.request(ctx, namespace, options)
	if err != nil {
		return err
	}
	if code != 0 {
		return errors.New("non success from probe command")
	}
	return nil
}

func (a *agentRunner) restartContainer(ctx context.Context, namespace types.Namespace) (*time.Time, error) {
	restartTimeout := int(time.Duration(0).Seconds())
	name := containerName(namespace)
	validUntil := time.Now().Add(a.o.AgentContainerLifeSpan)
	err := a.dockerClient.ContainerRestart(ctx, name, container.StopOptions{
		Timeout: &restartTimeout,
	})
	if err != nil {
		return nil, errors.Tag(err, "cannot restart container")
	}
	return &validUntil, nil
}

func (a *agentRunner) stopContainer(namespace types.Namespace) error {
	timeout := int(time.Duration(0).Seconds())
	err := a.dockerClient.ContainerStop(
		context.Background(),
		containerName(namespace),
		container.StopOptions{
			Timeout: &timeout,
		},
	)
	if err == nil {
		return nil
	}
	if errdefs.IsNotFound(err) {
		return nil
	}
	return errors.Tag(err, "cannot stop container")
}

func (a *agentRunner) request(ctx context.Context, namespace types.Namespace, options *types.CommandOptions) (types.ExitCode, error) {
	request := types.ExecAgentRequestOptions{
		CommandLine:        options.CommandLine,
		CommandOutputFiles: options.CommandOutputFiles,
		Environment:        options.Environment,
		ComputeTimeout:     options.ComputeTimeout,
	}
	socketPath := a.compileBaseDir.
		CompileDir(namespace).
		Join(sharedTypes.PathName(constants.AgentSocketName))

	conn, err := a.d.DialContext(ctx, "unix", socketPath)
	if err != nil {
		return -1, err
	}
	ctx, done := context.WithCancel(ctx)
	defer done()
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()
	err = conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	if err != nil {
		return -1, err
	}
	if err = json.NewEncoder(conn).Encode(request); err != nil {
		return -1, err
	}
	var body types.ExecAgentResponseBody
	if err = json.NewDecoder(conn).Decode(&body); err != nil {
		return -1, err
	}
	if options.Timed != nil {
		*options.Timed = body.Timed
	}
	if err = ctx.Err(); err != nil {
		return -1, err
	}

	switch body.ErrorMessage {
	case "":
		return body.ExitCode, nil
	case constants.Cancelled:
		return -1, context.Canceled
	case constants.TimedOut:
		return -1, context.DeadlineExceeded
	default:
		return -1, errors.New(body.ErrorMessage)
	}
}

func (a *agentRunner) Run(ctx context.Context, namespace types.Namespace, options *types.CommandOptions) (types.ExitCode, error) {
	code, err := a.request(ctx, namespace, options)
	if err != nil {
		// Ensure cleanup of any pending processes.
		_ = a.stopContainer(namespace)
		return -1, err
	}
	return code, nil
}

func (a *agentRunner) Resolve(path string, _ types.Namespace) (sharedTypes.PathName, error) {
	if strings.HasPrefix(path, constants.CompileDirContainer+"/") {
		return sharedTypes.PathName(path[len(constants.CompileDirContainer)+1:]), nil
	}
	if strings.HasPrefix(path, constants.OutputDirContainer+"/") {
		return sharedTypes.PathName(path[len(constants.OutputDirContainer)+1:]), nil
	}
	return "", errors.New("unknown base: " + path)
}
