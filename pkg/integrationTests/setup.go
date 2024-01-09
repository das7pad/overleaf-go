// Golang port of Overleaf
// Copyright (C) 2023-2024 Jakob Ackermann <das7pad@outlook.com>
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

package integrationTests

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	dockerTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/go-connections/nat"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/moby/term"

	configGenerator "github.com/das7pad/overleaf-go/cmd/config-generator/pkg/config-generator"
	minioSetup "github.com/das7pad/overleaf-go/cmd/minio-setup/pkg/minio-setup"
	"github.com/das7pad/overleaf-go/cmd/pkg/utils"
	"github.com/das7pad/overleaf-go/db"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/objectStorage"
)

var F configGenerator.Flags
var C configGenerator.Config

const (
	minioContainerName    = "ol-go-ci-minio"
	postgresContainerName = "ol-go-ci-pg"
	redisContainerName    = "ol-go-ci-redis"
	tmpPostgres           = "/tmp/" + postgresContainerName
	tmpRedis              = "/tmp/" + redisContainerName
	redisSocket           = tmpRedis + "/s"
)

func Setup(m *testing.M) {
	SetupFn(m, func() {})
}

func SetupFn(m *testing.M, setup func()) {
	code := 101
	defer func() {
		if err := recover(); err != nil {
			panic(err)
		}
		os.Exit(code)
	}()
	flag.Parse()
	if !testing.Verbose() {
		log.SetOutput(io.Discard)
	}

	F = configGenerator.NewFlags()
	F.AppName = "TESTING"
	F.BcryptCosts = 4 // faster registration/login
	F.ManifestPath = "empty"
	F.SMTPAddress = "discard"
	F.RealTimeWriteQueueDepth = 100

	dockerClient, dockerErr := client.NewClientWithOpts(
		client.WithHost(F.DockerHost()),
		client.WithAPIVersionNegotiation(),
	)
	if dockerErr != nil {
		panic(dockerErr)
	}

	ctx, done := context.WithCancel(context.Background())
	defer done()

	defer setupMinio(ctx, dockerClient)(&code)
	defer setupPg(ctx, dockerClient)(&code)
	defer setupRedis(ctx, dockerClient)(&code)

	// Pickup changes from minio/pg/redis setup
	C = configGenerator.Generate(F)
	C.PopulateEnv()

	setup()

	code = m.Run()
}

func setupMinio(ctx context.Context, c *client.Client) func(code *int) {
	i, err := createAndStartContainer(ctx, c, &container.Config{
		Env: []string{
			fmt.Sprintf("MINIO_ROOT_USER=%s", F.MinioRootUser),
			fmt.Sprintf("MINIO_ROOT_PASSWORD=%s", F.MinioRootPassword),
			fmt.Sprintf("MINIO_REGION=%s", F.FilestoreOptions.Region),
		},
		Cmd:   []string{"server", "--address", ":19000", "/data"},
		Image: "minio/minio:RELEASE.2023-07-07T07-13-57Z",
		ExposedPorts: map[nat.Port]struct{}{
			"19000/tcp": {},
		},
	}, &container.HostConfig{
		LogConfig:   container.LogConfig{Type: "json-file"},
		NetworkMode: "bridge",
		AutoRemove:  true,
		PortBindings: map[nat.Port][]nat.PortBinding{
			"19000/tcp": {{HostIP: "127.0.1.1", HostPort: "19000"}},
		},
	}, minioContainerName, []string{"1 Online"})
	if err != nil {
		panic(errors.Tag(err, "create minio container"))
	}

	for _, s := range i.Config.Env {
		_, v, found := strings.Cut(s, "MINIO_ROOT_USER=")
		if found {
			F.MinioRootUser = v
		}
		_, v, found = strings.Cut(s, "MINIO_ROOT_PASSWORD=")
		if found {
			F.MinioRootPassword = v
		}
	}
	t := strconv.FormatInt(time.Now().UnixNano(), 10)
	F.FilestoreOptions.Endpoint = getIP(i) + ":19000"
	bucket := "ci-bucket" + t
	F.FilestoreOptions.Bucket = bucket
	F.S3PolicyName = "ci-policy" + t

	o := minioSetup.Options{
		Endpoint:         F.FilestoreOptions.Endpoint,
		Secure:           F.FilestoreOptions.Secure,
		Region:           F.FilestoreOptions.Region,
		RootUser:         F.MinioRootUser,
		RootPassword:     F.MinioRootPassword,
		Bucket:           F.FilestoreOptions.Bucket,
		AccessKey:        F.FilestoreOptions.Key,
		SecretKey:        F.FilestoreOptions.Secret,
		PolicyName:       F.S3PolicyName,
		PolicyContent:    configGenerator.Generate(F).S3PolicyContent,
		CleanupOtherKeys: false,
	}
	for j := 0; j < 10; j++ {
		if err = minioSetup.Setup(ctx, o); err != nil {
			time.Sleep(time.Second)
			continue
		}
		break
	}
	if err != nil {
		panic(errors.Tag(err, "minio setup"))
	}

	b, err := objectStorage.FromOptions(F.FilestoreOptions)
	if err != nil {
		panic(errors.Tag(err, "create backend"))
	}

	go monitorContainer(ctx, c, minioContainerName)
	return func(code *int) {
		if *code != 0 {
			return
		}
		err = b.DeletePrefix(context.Background(), "/")
		if err != nil {
			panic(errors.Tag(err, "cleanup bucket"))
		}
	}
}

func buildPGDSN(dbName string) string {
	return fmt.Sprintf(
		"postgresql://postgres@/%s?host=%s&sslmode=disable",
		dbName, tmpPostgres,
	)
}

func setupPg(ctx context.Context, c *client.Client) func(code *int) {
	dbInit := path.Join(tmpPostgres, "ol-db-init")
	schema := path.Join(dbInit, "schema.sql")
	if _, err := os.Stat(schema); err != nil {
		if err = os.MkdirAll(dbInit, 0o777); err != nil {
			panic(errors.Tag(err, "create db init dir"))
		}
		err = os.WriteFile(schema, []byte(dbSchema.S), 0o444)
		if err != nil {
			panic(errors.Tag(err, "write schema"))
		}
	}

	_, err := createAndStartContainer(ctx, c, &container.Config{
		Env: []string{
			"POSTGRES_HOST_AUTH_METHOD=trust",
		},
		Cmd:   []string{"-c", "log_connections=yes"},
		Image: "postgres:14",
	}, &container.HostConfig{
		LogConfig:   container.LogConfig{Type: "json-file"},
		NetworkMode: "none",
		AutoRemove:  true,
		Mounts: []mount.Mount{
			{
				Type:     mount.TypeBind,
				Source:   dbInit,
				Target:   "/docker-entrypoint-initdb.d",
				ReadOnly: true,
			},
			{
				Type:   mount.TypeBind,
				Source: tmpPostgres,
				Target: "/var/run/postgresql",
			},
		},
	}, postgresContainerName, []string{
		"PostgreSQL init process complete; ready for start up.",
		"database system is ready to accept connections",
	})
	if err != nil {
		panic(errors.Tag(err, "create postgres container"))
	}

	dbName := "ci" + strconv.FormatInt(time.Now().UnixNano(), 10)

	for i := 0; i < 10; i++ {
		db, err2 := pgx.Connect(ctx, buildPGDSN("postgres"))
		if err2 != nil {
			err = errors.Tag(err2, "connect to pgx")
			time.Sleep(time.Second)
			continue
		}
		// NOTE: CREATE DATABASE does not support arguments.
		_, err2 = db.Exec(ctx, fmt.Sprintf(`
-- creating DBs from a template is very cheap + we can discard data when done
CREATE DATABASE %s WITH TEMPLATE postgres OWNER postgres
`, dbName))
		errClose := db.Close(ctx)
		if err2 == nil {
			// happy path
		} else if e, ok := err2.(*pgconn.PgError); ok && e.Code == "42P04" {
			// already exists
		} else {
			err = errors.Tag(err2, "copy db")
			time.Sleep(time.Second)
			continue
		}
		if errClose != nil {
			err = errors.Tag(errClose, "close db")
			time.Sleep(time.Second)
			continue
		}
		err = nil
		break
	}
	if err != nil {
		panic(err)
	}

	if err = os.Setenv("POSTGRES_DSN", buildPGDSN(dbName)); err != nil {
		panic(errors.Tag(err, "set POSTGRES_DSN"))
	}

	go monitorContainer(ctx, c, postgresContainerName)
	return func(code *int) {
		if *code != 0 {
			return
		}
		db, err2 := pgx.Connect(context.Background(), buildPGDSN("postgres"))
		if err2 != nil {
			panic(errors.Tag(err2, "connect to pgx"))
		}
		// NOTE: DROP DATABASE does not support arguments.
		_, err2 = db.Exec(context.Background(), fmt.Sprintf(`
-- FORCE terminates any pending connections
DROP DATABASE %s WITH (FORCE)
`, dbName))
		errClose := db.Close(context.Background())
		if err2 != nil {
			panic(errors.Tag(err2, "drop db"))
		}
		if errClose != nil {
			panic(errors.Tag(errClose, "close db"))
		}
	}
}

func setupRedis(ctx context.Context, c *client.Client) func(code *int) {
	if err := os.MkdirAll(tmpRedis, 0o777); err != nil {
		panic(errors.Tag(err, "create redis dir"))
	}
	_, err := createAndStartContainer(ctx, c, &container.Config{
		Entrypoint: []string{
			"redis-server", "--databases", "1024",
			"--unixsocket", redisSocket, "--unixsocketperm", "777",
		},
		Image: "redis:6",
	}, &container.HostConfig{
		LogConfig:   container.LogConfig{Type: "json-file"},
		NetworkMode: "none",
		AutoRemove:  true,
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: tmpRedis,
				Target: tmpRedis,
			},
		},
	}, redisContainerName, []string{"Ready to accept connections"})
	if err != nil {
		panic(errors.Tag(err, "create redis container"))
	}

	if err = os.Setenv("REDIS_HOST", redisSocket); err != nil {
		panic(errors.Tag(err, "set REDIS_HOST"))
	}

	var rClient redis.UniversalClient
	for db := 0; db < 1024; db++ {
		s := strconv.FormatInt(int64(db), 10)
		if err = os.Setenv("REDIS_DB", s); err != nil {
			panic(errors.Tag(err, "set REDIS_DB"))
		}
		rClient = utils.MustConnectRedis(ctx)
		cmd := rClient.SetNX(ctx, "claim", 1, time.Hour*24)
		if cmd.Err() != nil || cmd.Val() != true {
			if err = rClient.Close(); err != nil {
				panic(errors.Tag(err, "close redis client"))
			}
			rClient = nil
			continue
		}
		break
	}
	if rClient == nil {
		panic(errors.New("all redis databases are taken"))
	}

	go monitorContainer(ctx, c, redisContainerName)
	return func(code *int) {
		if *code != 0 {
			return
		}
		if err = rClient.FlushDB(context.Background()).Err(); err != nil {
			panic(errors.Tag(err, "cleanup redis"))
		}
		if err = rClient.Close(); err != nil {
			panic(errors.Tag(err, "close redis client"))
		}
	}
}

func getIP(i *dockerTypes.ContainerJSON) string {
	if F.DockerSocketRootful != F.DockerSocket {
		return "127.0.1.1"
	}
	return i.NetworkSettings.Networks["bridge"].IPAddress
}

func createAndStartContainer(ctx context.Context, c *client.Client, containerConfig *container.Config, hostConfig *container.HostConfig, name string, msg []string) (*dockerTypes.ContainerJSON, error) {
	var i *dockerTypes.ContainerJSON
	var err error
	for j := 0; j < 5; j++ {
		i, err = createAndStartContainerOnce(ctx, c, containerConfig, hostConfig, name)
		if err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		return nil, err
	}
	if err = waitForContainerLogMessage(ctx, c, name, msg); err != nil {
		return nil, err
	}
	return i, err
}

func createAndStartContainerOnce(ctx context.Context, c *client.Client, containerConfig *container.Config, hostConfig *container.HostConfig, name string) (*dockerTypes.ContainerJSON, error) {
	i, err := c.ContainerInspect(ctx, name)
	if err == nil && i.State != nil && i.State.Running {
		t, err2 := time.Parse(time.RFC3339, i.State.StartedAt)
		if err2 != nil {
			return nil, errors.Tag(err2, "parse startedAt")
		}
		if time.Now().Sub(t) < 24*time.Hour {
			return &i, nil
		}
		if err = c.ContainerKill(ctx, name, "KILL"); err != nil {
			return nil, errors.Tag(err, "kill old container")
		}
		res, errs := c.ContainerWait(ctx, name, container.WaitConditionRemoved)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-res:
		case err = <-errs:
			if err != nil && !errdefs.IsNotFound(err) {
				return nil, errors.Tag(err, "wait for container removal")
			}
		}
	}

	_, err = c.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, name)
	switch {
	case err == nil:
		// happy path
	case errdefs.IsConflict(err):
		// already created
	case errdefs.IsNotFound(err):
		// missing image
		r, err2 := c.ImagePull(
			ctx, containerConfig.Image, dockerTypes.ImagePullOptions{},
		)
		if err2 != nil {
			return nil, errors.Tag(err2, "initiate pull")
		}
		fd, isTerminal := term.GetFdInfo(os.Stderr)
		err2 = jsonmessage.DisplayJSONMessagesStream(
			r, os.Stderr, fd, isTerminal, nil,
		)
		if err2 != nil {
			return nil, errors.Tag(err2, "stream pull response")
		}
		if err2 = r.Close(); err2 != nil {
			return nil, errors.Tag(err2, "close pull response")
		}
		_, err2 = c.ContainerCreate(
			ctx, containerConfig, hostConfig, nil, nil, name,
		)
		if err2 != nil && !errdefs.IsConflict(err2) {
			return nil, errors.Tag(err2, "create container")
		}
	default:
		return nil, errors.Tag(err, "create container")
	}

	err = c.ContainerStart(ctx, name, dockerTypes.ContainerStartOptions{})
	switch {
	case err == nil:
		// happy path
	case errdefs.IsConflict(err):
		// already running
	default:
		return nil, errors.Tag(err, "start container")
	}

	for j := 0; j < 10; j++ {
		i, err = c.ContainerInspect(ctx, name)
		if err != nil {
			return nil, errors.Tag(err, "inspect container")
		}
		if i.State != nil && i.State.Running {
			return &i, nil
		}
		time.Sleep(time.Second)
	}

	return nil, errors.New("container not running yet: " + name)
}

func monitorContainer(ctx context.Context, c *client.Client, id string) {
	res, errs := c.ContainerWait(context.Background(), id, container.WaitConditionNotRunning)

	select {
	case <-ctx.Done():
	case <-res:
		panic(errors.New("container exited early"))
	case err := <-errs:
		if err != nil {
			panic(errors.Tag(err, "wait for container"))
		}
	}
}

func waitForContainerLogMessage(ctx context.Context, c *client.Client, id string, msg []string) error {
	delay := 100 * time.Millisecond
	for i := 0; i < 20; i++ {
		r, err := c.ContainerLogs(ctx, id, dockerTypes.ContainerLogsOptions{
			ShowStderr: true,
			ShowStdout: true,
		})
		if err != nil {
			return errors.Tag(err, "get container logs")
		}
		blob, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			return errors.Tag(err, "consume container logs")
		}
		idx := 0
		for _, s := range msg {
			if idx = bytes.Index(blob[idx:], []byte(s)); idx == -1 {
				break
			}
		}
		if idx != -1 {
			return nil
		}
		log.Printf("waiting %s for %q to emit %q", delay, id, msg)
		time.Sleep(delay)
		delay += 100 * time.Millisecond
	}
	return errors.New(fmt.Sprintf("container did not emit %q", msg))
}
