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

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/das7pad/overleaf-go/pkg/email"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	webTypes "github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func main() {
	toRaw := flag.String("to", "", "recipient of the email")
	timeout := flag.Duration("timout", 10*time.Second, "timeout for operation")
	flag.Parse()
	if *toRaw == "" {
		fmt.Println("ERR: must set -to")
		flag.Usage()
		os.Exit(101)
	}
	to := sharedTypes.Email(*toRaw).Normalize()
	if err := to.Validate(); err != nil {
		fmt.Println(errors.Tag(err, "ERR: invalid email address").Error())
		flag.Usage()
		os.Exit(101)
	}

	o := webTypes.Options{}
	o.FillFromEnv()
	if err := o.Validate(); err != nil {
		panic(errors.Tag(err, "WEB_OPTIONS invalid"))
	}
	emailOptions := o.EmailOptions()

	ctx, done := context.WithTimeout(context.Background(), *timeout)
	defer done()

	e := email.Email{
		Content: &email.CTAContent{
			PublicOptions: emailOptions.Public,
			Message: email.Message{
				"This is a test Email from " + o.AppName,
			},
			Title:   "A Test Email from " + o.AppName,
			CTAText: "Open " + o.AppName,
			CTAURL:  &o.SiteURL,
		},
		Subject: "A Test Email from " + o.AppName,
		To: email.Identity{
			Address: to,
		},
	}
	log.Printf("sending to %q", to)
	if err := e.Send(ctx, emailOptions.Send); err != nil {
		panic(errors.Tag(err, "send email"))
	}
	log.Println("sent.")
}
