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

package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type SMTPAddress string

func (a SMTPAddress) Host() string {
	s := string(a)
	idx := strings.LastIndexByte(s, ':')
	return s[0:idx]
}

func (a SMTPAddress) Validate() error {
	if a == "log" {
		return nil
	}
	if !strings.ContainsRune(string(a), ':') {
		return &errors.ValidationError{Msg: "missing port spec"}
	}
	return nil
}

type SendOptions struct {
	From            Identity
	FallbackReplyTo Identity
	SMTPAddress     SMTPAddress
	SMTPHello       string
	SMTPAuth        smtp.Auth
}

type generator func(w io.Writer) error

const (
	crlf = "\r\n"

	htmlContent      = "text/html"
	plainTextContent = "text/plain"
)

func writePart(m *multipart.Writer, contentType string, gen generator) error {
	h := textproto.MIMEHeader{
		"Content-Transfer-Encoding": {"quoted-printable"},
		"Content-Type":              {contentType + "; charset=UTF-8"},
	}
	p, err := m.CreatePart(h)
	if err != nil {
		return errors.Tag(err, "cannot create new part")
	}
	q := quotedprintable.NewWriter(p)
	err = gen(q)
	errClose := q.Close()
	if err != nil {
		return errors.Tag(err, "cannot generate content")
	}
	if errClose != nil {
		return errors.Tag(errClose, "cannot finalize part")
	}
	return nil
}

func (e *Email) Send(ctx context.Context, o *SendOptions) error {
	if err := e.Validate(); err != nil {
		return err
	}

	replyTo := o.FallbackReplyTo
	if e.ReplyTo.Address != "" {
		replyTo = e.ReplyTo
	}

	// A minimal CTA email weights 27 KB, use a larger value to avoid growing.
	b := bytes.NewBuffer(make([]byte, 0, 30*1024))

	w := b
	m := multipart.NewWriter(w)
	rndHex := m.Boundary()

	// The body parts are 'quoted-printable' encoded. The encoding uses '=' for
	//  denoting forced line breaks and encoding non-ASCII characters. It
	//  encodes the literal character '=' as '=3D'.
	// It is hence impossible to get the sequence '==' in the encoded output.
	if err := m.SetBoundary("==" + rndHex + "=="); err != nil {
		return errors.Tag(err, "cannot set robust boundary")
	}

	now := time.Now()
	headers := map[string]string{
		"Content-Type": fmt.Sprintf(
			"multipart/alternative; boundary=%q", m.Boundary(),
		),
		"Date": now.Format(time.RFC1123Z),
		"From": o.From.String(),
		"Message-Id": fmt.Sprintf(
			"<%x-%s@%s>", now.UnixNano(), rndHex, o.From.Address.Host(),
		),
		"MIME-Version": "1.0",
		"Reply-To":     replyTo.String(),
		"Subject":      mime.QEncoding.Encode("UTF-8", e.Subject),
		"To":           e.To.String(),
	}
	for k, s := range headers {
		if _, err := io.WriteString(w, k+": "+s+crlf); err != nil {
			return errors.Tag(err, "cannot write header")
		}
	}

	if _, err := io.WriteString(w, crlf); err != nil {
		return errors.Tag(err, "cannot write start of body")
	}
	if err := writePart(m, plainTextContent, e.writePlainText); err != nil {
		return errors.Tag(err, "cannot write plain text part")
	}
	if err := writePart(m, htmlContent, e.writeHTML); err != nil {
		return errors.Tag(err, "cannot write html part")
	}

	if err := m.Close(); err != nil {
		return errors.Tag(err, "cannot finalize body")
	}

	if o.SMTPAddress == "log" {
		log.Println(w.String())
		return nil
	}
	err := sendMail(
		ctx, o.SMTPAddress, o.SMTPAuth, o.SMTPHello, o.From, e.To, b.Bytes(),
	)
	if err != nil {
		log.Printf("cannot send email: %s", err)
		// Ensure that we do not expose details on the email infrastructure.
		return errors.New("cannot send email")
	}
	return nil
}

func sendMail(ctx context.Context, addr SMTPAddress, a smtp.Auth, hello string, from, to Identity, blob []byte) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", string(addr))
	if err != nil {
		return errors.Tag(err, "connect")
	}
	ctx, done := context.WithCancel(ctx)
	defer done()
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()
	c, err := smtp.NewClient(conn, addr.Host())
	if err != nil {
		return errors.Tag(err, "create client")
	}
	defer func() { _ = c.Close() }()

	if err = c.Hello(hello); err != nil {
		return errors.Tag(err, "hello")
	}
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err = c.StartTLS(&tls.Config{ServerName: addr.Host()}); err != nil {
			return errors.Tag(err, "starttls")
		}
	}
	if a != nil {
		if ok, _ := c.Extension("AUTH"); !ok {
			return errors.New("expected AUTH support")
		}
		if err = c.Auth(a); err != nil {
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
