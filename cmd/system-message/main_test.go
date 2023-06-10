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
	"flag"
	"os"
	"testing"

	"github.com/das7pad/overleaf-go/cmd/pkg/utils"
	"github.com/das7pad/overleaf-go/pkg/integrationTests"
	"github.com/das7pad/overleaf-go/pkg/models/systemMessage"
)

func TestMain(m *testing.M) {
	integrationTests.Setup(m)
}

func TestMainFn(t *testing.T) {
	os.Args = []string{"exec", "--message=MSG"}
	main()

	ctx := context.Background()

	db := utils.MustConnectPostgres(ctx)
	smm := systemMessage.New(db)
	m, err := smm.GetAll(ctx)
	if err != nil {
		t.Fatalf("get all: %s", err)
	}
	if len(m) != 1 || m[0].Content != "MSG" {
		t.Fatalf("get all: %#v", m)
	}

	os.Args = []string{"exec", "--clear"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	main()

	m, err = smm.GetAll(ctx)
	if err != nil {
		t.Fatalf("get all again: %s", err)
	}
	if len(m) != 0 {
		t.Fatalf("get all again: %#v", m)
	}
}
