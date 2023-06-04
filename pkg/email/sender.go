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

package email

import (
	"context"
	"crypto/tls"
	"log"
	"net"
	"net/smtp"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type Sender interface {
	Send(ctx context.Context, from, to Identity, blob []byte) error
}

func NewSender(address SMTPAddress, smtpHello string, smtpAuth smtp.Auth) Sender {
	switch address {
	case "log":
		return loggingSender{}
	default:
		return smtpSender{
			addr:  address,
			auth:  smtpAuth,
			hello: smtpHello,
		}
	}
}

type loggingSender struct {
}

func (l loggingSender) Send(_ context.Context, _, _ Identity, blob []byte) error {
	log.Println(string(blob))
	return nil
}

type smtpSender struct {
	addr  SMTPAddress
	auth  smtp.Auth
	hello string
}

func (s smtpSender) Send(ctx context.Context, from, to Identity, blob []byte) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", string(s.addr))
	if err != nil {
		return errors.Tag(err, "connect")
	}
	ctx, done := context.WithCancel(ctx)
	defer done()
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()
	c, err := smtp.NewClient(conn, s.addr.Host())
	if err != nil {
		return errors.Tag(err, "create client")
	}
	defer func() { _ = c.Close() }()

	if err = c.Hello(s.hello); err != nil {
		return errors.Tag(err, "hello")
	}
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err = c.StartTLS(&tls.Config{ServerName: s.addr.Host()}); err != nil {
			return errors.Tag(err, "starttls")
		}
	}
	if s.auth != nil {
		if ok, _ := c.Extension("AUTH"); !ok {
			return errors.New("expected AUTH support")
		}
		if err = c.Auth(s.auth); err != nil {
			return errors.Tag(err, "auth")
		}
	}
	if err = c.Mail(string(from.Address)); err != nil {
		return errors.Tag(err, "mail")
	}
	if err = c.Rcpt(string(to.Address)); err != nil {
		return errors.Tag(err, "receipt")
	}
	w, err := c.Data()
	if err != nil {
		return errors.Tag(err, "data")
	}
	if _, err = w.Write(blob); err != nil {
		return errors.Tag(err, "write")
	}
	if err = w.Close(); err != nil {
		return errors.Tag(err, "flush write")
	}
	if err = c.Quit(); err != nil {
		return errors.Tag(err, "quit")
	}
	return nil
}
