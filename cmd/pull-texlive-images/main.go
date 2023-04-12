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
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	dockerTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/moby/term"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

func main() {
	ctx, triggerExit := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM,
	)
	defer triggerExit()
	o := clsiTypes.Options{}
	o.FillFromEnv()

	c, dockerErr := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if dockerErr != nil {
		panic(dockerErr)
	}

	for _, image := range o.AllowedImages {
		if createTestContainer(ctx, c, image) == nil {
			log.Printf("%s: already exists", image)
			continue
		}

		log.Printf("%s: starting to pull, this can take a while!", image)
		pullOptions := dockerTypes.ImagePullOptions{}
		r, err := c.ImagePull(ctx, string(image), pullOptions)
		if err != nil {
			panic(errors.Tag(err, "initiate pull"))
		}

		fd, isTerminal := term.GetFdInfo(os.Stdout)
		err = jsonmessage.DisplayJSONMessagesStream(
			r, os.Stdout, fd, isTerminal, nil,
		)
		if err != nil {
			panic(errors.Tag(err, "stream pull response"))
		}
		if err = r.Close(); err != nil {
			panic(errors.Tag(err, "close pull response"))
		}
	}

	log.Println("Done pulling.")
}

func createTestContainer(ctx context.Context, c *client.Client, imageName sharedTypes.ImageName) error {
	_, err := c.ContainerCreate(ctx,
		&container.Config{
			Cmd:             make([]string, 0),
			Image:           string(imageName),
			WorkingDir:      "/",
			Entrypoint:      []string{"/usr/bin/true"},
			NetworkDisabled: true,
		},
		&container.HostConfig{
			LogConfig: container.LogConfig{
				Type: "none",
			},
			NetworkMode: "none",
			AutoRemove:  true,
		},
		nil,
		nil,
		"",
	)
	return err
}
