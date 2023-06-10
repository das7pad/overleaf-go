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
	"os"
	"testing"

	"github.com/das7pad/overleaf-go/cmd/pkg/utils"
	"github.com/das7pad/overleaf-go/pkg/integrationTests"
	"github.com/das7pad/overleaf-go/pkg/models/user"
)

func TestMain(m *testing.M) {
	integrationTests.Setup(m)
}

func TestMainFn(t *testing.T) {
	const email = "foo@bar.com"

	os.Args = []string{"exec", "--email=foo@bar.com"}
	main()

	ctx := context.Background()

	db := utils.MustConnectPostgres(ctx)
	um := user.New(db)
	u := user.WithPublicInfo{}
	if err := um.GetUserByEmail(ctx, email, &u); err != nil {
		t.Fatalf("find user by email: %s", err)
	}
}
