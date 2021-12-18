// Golang port of Overleaf
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
	"context"
	"encoding/json"
	"flag"
	"log"
	"time"

	"github.com/das7pad/overleaf-go/cmd/internal/utils"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/pubSub/channel"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func main() {
	delay := flag.Duration("delay", 10*time.Second, "delay for forced page reload")
	timeout := flag.Duration("timout", 10*time.Second, "timeout for operation")
	flag.Parse()

	client := utils.MustConnectRedis(*timeout)
	editorEvents := channel.NewWriter(client, "editor-events")

	ctx, done := context.WithTimeout(context.Background(), *timeout)
	defer done()

	payload, err := json.Marshal([]interface{}{"not-used", delay.Seconds()})
	if err != nil {
		panic(errors.Tag(err, "cannot serialize payload"))
	}
	log.Println("Broadcasting message.")
	err = editorEvents.Publish(ctx, &sharedTypes.EditorEventsMessage{
		Message: "forceDisconnect",
		Payload: payload,
	})
	if err != nil {
		panic(errors.Tag(err, "cannot broadcast message"))
	}
	log.Println("done.")
}
