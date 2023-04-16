// Golang port of Overleaf
// Copyright (C) 2022-2023 Jakob Ackermann <das7pad@outlook.com>
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
	"os"
	"os/signal"
	"syscall"

	"github.com/das7pad/overleaf-go/cmd/pkg/utils"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web"
	webTypes "github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func main() {
	ctx, done := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer done()

	var email sharedTypes.Email
	flag.StringVar((*string)(&email), "email", "", "new users email")
	var initiatorUserIdRaw string
	flag.StringVar(&initiatorUserIdRaw, "initiator-user-id", sharedTypes.AllZeroUUID, "optional user-id of the command line operator for leaving an audit log trail")
	var quiet bool
	flag.BoolVar(&quiet, "quiet", false, "just print the url on success")

	flag.Parse()
	if err := email.Validate(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "ERR: %s\n", err.Error())
		flag.Usage()
		os.Exit(1)
	}
	var initiatorUserId sharedTypes.UUID
	if initiatorUserIdRaw != sharedTypes.AllZeroUUID {
		var err error
		initiatorUserId, err = sharedTypes.ParseUUID(initiatorUserIdRaw)
		if err != nil {
			err = errors.Tag(err, "invalid initiator-user-id")
			_, _ = fmt.Fprintf(os.Stderr, "ERR: %s\n", err.Error())
			flag.Usage()
			os.Exit(1)
		}
	}

	db := utils.MustConnectPostgres(ctx)
	rClient := utils.MustConnectRedis(ctx)
	addr := "127.0.0.1:3000"
	localURL := "http://" + addr

	webOptions := webTypes.Options{}
	webOptions.FillFromEnv()
	webManager, err := web.New(&webOptions, db, rClient, localURL, nil, nil)
	if err != nil {
		panic(errors.Tag(err, "web setup"))
	}

	req := webTypes.NewCMDCreateUserRequest(email, initiatorUserId)
	res := webTypes.CMDCreateUserResponse{}
	if err = webManager.CMDCreateUser(ctx, &req, &res); err != nil {
		panic(errors.Tag(err, "cannot create user"))
	}
	if quiet {
		fmt.Println(res.SetNewPasswordURL)
		return
	}
	fmt.Printf(`
-------------------------------------------------------------------------------

    We've sent out a welcome email to the newly registered user.

    You can also manually send them the below URL to allow them to set their
     password and log in for the first time.

    (User activate tokens will expire after one week and the user will need
     to request a new password via the "forgot password" process, which they
     can initiate from the login screen.)

    Email: %s
    URL:   %s

-------------------------------------------------------------------------------
`, email, res.SetNewPasswordURL)
}
