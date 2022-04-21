// Golang port of Overleaf
// Copyright (C) 2021-2022 Jakob Ackermann <das7pad@outlook.com>
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
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/das7pad/overleaf-go/cmd/internal/utils"
	"github.com/das7pad/overleaf-go/pkg/models/systemMessage"
)

func main() {
	clear := flag.Bool("clear", false, "clear system messages")
	msg := flag.String("message", "", "create new system message")
	timeout := flag.Duration("timout", 10*time.Second, "timeout for operation")
	flag.Parse()
	*msg = strings.TrimSpace(*msg)

	c := utils.MustConnectEdgedb(*timeout)
	smm := systemMessage.New(c)

	ctx, done := context.WithTimeout(context.Background(), *timeout)
	defer done()
	var err error
	if *clear {
		log.Println("Deleting all system messages.")
		err = smm.DeleteAll(ctx)
	} else if *msg != "" {
		log.Println("Creating new system message.")
		err = smm.Create(ctx, *msg)
	} else {
		var messages []systemMessage.Full
		messages, err = smm.GetAll(ctx)
		if err == nil {
			for i, message := range messages {
				fmt.Printf("%d: %s: %s\n", i, message.Id, message.Content)
			}
		}
	}
	if err != nil {
		panic(err)
	}
	log.Println("done.")
}
