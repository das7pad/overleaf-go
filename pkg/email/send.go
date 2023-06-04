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
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
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

func (a SMTPAddress) IsSpecial() bool {
	switch a {
	case "collect", "discard", "log":
		return true
	default:
		return false
	}
}

func (a SMTPAddress) Validate() error {
	if a.IsSpecial() {
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
	Sender          Sender
}

type generator func(w io.Writer) error

const (
	colonSpace = ": "
	crlf       = "\r\n"

	htmlContent      = "text/html; charset=UTF-8"
	plainTextContent = "text/plain; charset=UTF-8"
)

func writePart(m *multipart.Writer, contentType string, gen generator) error {
	h := textproto.MIMEHeader{
		"Content-Transfer-Encoding": {"quoted-printable"},
		"Content-Type":              {contentType},
	}
	p, err := m.CreatePart(h)
	if err != nil {
		return errors.Tag(err, "create new part")
	}
	q := quotedprintable.NewWriter(p)
	err = gen(q)
	errClose := q.Close()
	if err != nil {
		return errors.Tag(err, "generate content")
	}
	if errClose != nil {
		return errors.Tag(errClose, "finalize part")
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

	m := multipart.NewWriter(b)
	rndHex := m.Boundary()

	// The body parts are 'quoted-printable' encoded. The encoding uses '=' for
	//  denoting forced line breaks and encoding non-ASCII characters. It
	//  encodes the literal character '=' as '=3D'.
	// It is hence impossible to get the sequence '==' in the encoded output.
	if err := m.SetBoundary("==" + rndHex + "=="); err != nil {
		return errors.Tag(err, "set robust boundary")
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
		b.WriteString(k)
		b.WriteString(colonSpace)
		b.WriteString(s)
		b.WriteString(crlf)
	}

	if _, err := b.WriteString(crlf); err != nil {
		return errors.Tag(err, "write start of body")
	}
	if err := writePart(m, plainTextContent, e.writePlainText); err != nil {
		return errors.Tag(err, "write plain text part")
	}
	if err := writePart(m, htmlContent, e.writeHTML); err != nil {
		return errors.Tag(err, "write html part")
	}

	if err := m.Close(); err != nil {
		return errors.Tag(err, "finalize body")
	}

	if err := o.Sender.Send(ctx, o.From, e.To, b.Bytes()); err != nil {
		log.Printf("send email: %s", err)
		// Ensure that we do not expose details on the email infrastructure.
		return errors.New("internal error sending email")
	}
	return nil
}
