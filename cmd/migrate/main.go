// Golang port of Overleaf
// Copyright (C) 2022 Jakob Ackermann <das7pad@outlook.com>
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
	"log"
	"os"
	"time"

	"github.com/das7pad/overleaf-go/cmd/internal/utils"
)

func main() {
	timeoutConnect := flag.Duration("timoutConnect", 10*time.Second, "timeout for connecting to db")
	flag.Parse()

	db := utils.MustConnectMongo(*timeoutConnect)
	m := New(db)

	ctx := context.Background()
	ok := false
	switch flag.Arg(0) {
	case "list":
		ok = m.List(ctx)
	case "migrate":
		ok = m.Migrate(ctx)
	default:
		log.Println("ERR: unknown sub command, use 'list' or 'migrate'")
		ok = false
	}
	if !ok {
		os.Exit(1)
	}
}
