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
	"flag"
	"fmt"
	"log"
	"os"
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
	if *clear == false && *msg == "" {
		fmt.Println("ERR: must set -clear or -message")
		flag.Usage()
		os.Exit(101)
	}

	db := utils.MustConnectMongo(*timeout)
	smm := systemMessage.New(db)

	ctx, done := context.WithTimeout(context.Background(), *timeout)
	defer done()
	var err error
	if *clear {
		log.Println("Deleting all system messages.")
		err = smm.DeleteAll(ctx)
	} else {
		log.Println("Creating new system message.")
		err = smm.Create(ctx, *msg)
	}
	if err != nil {
		panic(err)
	}
	log.Println("done.")
}
