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

package email

import (
	"context"
	"net/mail"
	"reflect"
	"testing"
	"time"

	"github.com/das7pad/overleaf-go/pkg/email/pkg/gmailGoToAction"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func TestEmail_Send(t *testing.T) {
	now := time.Unix(1, 0)
	po := &PublicOptions{
		AppName:          "Test App Name",
		CustomFooter:     "Custom Plain Text Footer",
		CustomFooterHTML: "Custom HTML Footer",
		SiteURL:          "https://example.com",
	}
	soBase := SendOptions{
		From: Identity{
			Address:     "from@example.com",
			DisplayName: "From-Name",
		},
		FallbackReplyTo: Identity{
			Address:     "fallback-reply-to@example.com",
			DisplayName: "Fallback-Reply-To-Name",
		},
	}
	hl1 := HelpLink{
		Before: "Before Help Link",
		Label:  "Help Link Label",
		After:  "After Help Link",
	}
	hl1.URL, _ = sharedTypes.ParseAndValidateURL("https://example.com/learn")
	ctaURL, _ := sharedTypes.ParseAndValidateURL("https://example.com/cta")
	type fields struct {
		Content  Content
		ReplyTo  Identity
		Subject  string
		To       Identity
		boundary string
		now      time.Time
	}
	type output struct {
		Headers   mail.Header
		PlainText string
	}
	tests := []struct {
		name    string
		fields  fields
		output  output
		wantErr bool
	}{
		{
			name: "failed validation",
			fields: fields{
				Content: &NoCTAContent{},
			},
			wantErr: true,
		},
		{
			name: "simple no cta",
			fields: fields{
				Content: &NoCTAContent{
					PublicOptions: po,
					Message:       []string{"line1", "line2"},
					Title:         "Email Title",
					HelpLinks:     []HelpLink{hl1},
				},
				ReplyTo: Identity{
					Address:     "reply-to@example.com",
					DisplayName: "Reply-To-Name",
				},
				Subject: "Email Subject",
				To: Identity{
					Address:     "to@example.com",
					DisplayName: "To-Name",
				},
				boundary: "random-boundary",
				now:      now,
			},
			output: output{
				Headers: mail.Header{
					"Content-Type": []string{
						`multipart/alternative; boundary="==random-boundary=="`,
					},
					"Date": []string{"Thu, 01 Jan 1970 00:00:01 +0000"},
					"From": []string{`"From-Name" <from@example.com>`},
					"Message-Id": []string{
						"<3b9aca00-random-boundary@example.com>",
					},
					"Mime-Version": []string{"1.0"},
					"Reply-To": []string{
						`"Reply-To-Name" <reply-to@example.com>`,
					},
					"Subject": []string{"Email Subject"},
					"To":      []string{`"To-Name" <to@example.com>`},
				},
				PlainText: "Hi,\r\n\r\nline1\r\n\r\nline2\r\n\r\nBefore Help LinkHelp Link Label (https://example.com/learn)After Help Link\r\n\r\nRegards,\r\nThe Test App Name Team - https://example.com\r\n\r\nCustom Plain Text Footer",
			},
		},
		{
			name: "simple cta",
			fields: fields{
				Content: &CTAContent{
					PublicOptions:    po,
					Message:          []string{"primary", "message"},
					SecondaryMessage: []string{"secondary", "message"},
					Title:            "Email Title",
					HelpLinks:        []HelpLink{hl1},
					CTAIntro:         "CTA-Intro",
					CTAText:          "CTA-Text",
					CTAURL:           ctaURL,
					GmailGoToAction: &gmailGoToAction.GmailGoToAction{
						Target:      ctaURL,
						Name:        "Gmail-Name",
						Description: "Gmail-Description",
					},
				},
				ReplyTo: Identity{
					Address:     "reply-to@example.com",
					DisplayName: "Reply-To-Name",
				},
				Subject: "Email Subject",
				To: Identity{
					Address:     "to@example.com",
					DisplayName: "To-Name",
				},
				boundary: "random-boundary",
				now:      now,
			},
			output: output{
				Headers: mail.Header{
					"Content-Type": []string{
						`multipart/alternative; boundary="==random-boundary=="`,
					},
					"Date": []string{"Thu, 01 Jan 1970 00:00:01 +0000"},
					"From": []string{`"From-Name" <from@example.com>`},
					"Message-Id": []string{
						"<3b9aca00-random-boundary@example.com>",
					},
					"Mime-Version": []string{"1.0"},
					"Reply-To": []string{
						`"Reply-To-Name" <reply-to@example.com>`,
					},
					"Subject": []string{"Email Subject"},
					"To":      []string{`"To-Name" <to@example.com>`},
				},
				PlainText: "Hi,\r\n\r\nprimary\r\n\r\nmessage\r\n\r\nBefore Help LinkHelp Link Label (https://example.com/learn)After Help Link\r\n\r\nCTA-Text: https://example.com/cta\r\n\r\nsecondary\r\n\r\nmessage\r\n\r\nRegards,\r\nThe Test App Name Team - https://example.com\r\n\r\nCustom Plain Text Footer",
			},
		},
		{
			name: "fallback reply-to",
			fields: fields{
				Content: &NoCTAContent{
					PublicOptions: po,
					Message:       []string{"line1", "line2"},
					Title:         "Email Title",
					HelpLinks:     []HelpLink{hl1},
				},
				Subject: "Email Subject",
				To: Identity{
					Address:     "to@example.com",
					DisplayName: "To-Name",
				},
				boundary: "random-boundary",
				now:      now,
			},
			output: output{
				Headers: mail.Header{
					"Content-Type": []string{
						`multipart/alternative; boundary="==random-boundary=="`,
					},
					"Date": []string{"Thu, 01 Jan 1970 00:00:01 +0000"},
					"From": []string{`"From-Name" <from@example.com>`},
					"Message-Id": []string{
						"<3b9aca00-random-boundary@example.com>",
					},
					"Mime-Version": []string{"1.0"},
					"Reply-To": []string{
						`"Fallback-Reply-To-Name" <fallback-reply-to@example.com>`,
					},
					"Subject": []string{"Email Subject"},
					"To":      []string{`"To-Name" <to@example.com>`},
				},
				PlainText: "Hi,\r\n\r\nline1\r\n\r\nline2\r\n\r\nBefore Help LinkHelp Link Label (https://example.com/learn)After Help Link\r\n\r\nRegards,\r\nThe Test App Name Team - https://example.com\r\n\r\nCustom Plain Text Footer",
			},
		},
		{
			name: "umlauts",
			fields: fields{
				Content: &NoCTAContent{
					PublicOptions: po,
					Message:       []string{"line1 ä", "line2 ö"},
					Title:         "Title ü",
					HelpLinks:     []HelpLink{hl1},
				},
				ReplyTo: Identity{
					Address:     "reply-to@example.com",
					DisplayName: "Reply-To-Name Ä",
				},
				Subject: "Email Subject Ö",
				To: Identity{
					Address:     "to@example.com",
					DisplayName: "To-Name Ü",
				},
				boundary: "random-boundary",
				now:      now,
			},
			output: output{
				Headers: mail.Header{
					"Content-Type": []string{
						`multipart/alternative; boundary="==random-boundary=="`,
					},
					"Date": []string{"Thu, 01 Jan 1970 00:00:01 +0000"},
					"From": []string{`"From-Name" <from@example.com>`},
					"Message-Id": []string{
						"<3b9aca00-random-boundary@example.com>",
					},
					"Mime-Version": []string{"1.0"},
					"Reply-To": []string{
						`=?utf-8?q?Reply-To-Name_=C3=84?= <reply-to@example.com>`,
					},
					"Subject": []string{"=?UTF-8?q?Email_Subject_=C3=96?="},
					"To":      []string{`=?utf-8?q?To-Name_=C3=9C?= <to@example.com>`},
				},
				PlainText: "Hi,\r\n\r\nline1 ä\r\n\r\nline2 ö\r\n\r\nBefore Help LinkHelp Link Label (https://example.com/learn)After Help Link\r\n\r\nRegards,\r\nThe Test App Name Team - https://example.com\r\n\r\nCustom Plain Text Footer",
			},
		},
	}
	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			so := soBase
			cs := collectingSender{}
			so.Sender = &cs
			e := &Email{
				Content:  tt.fields.Content,
				ReplyTo:  tt.fields.ReplyTo,
				Subject:  tt.fields.Subject,
				To:       tt.fields.To,
				boundary: tt.fields.boundary,
				now:      tt.fields.now,
			}
			if err := e.Send(ctx, &so); (err != nil) != tt.wantErr {
				t.Errorf("Send() error = %v, wantErr %v", err, tt.wantErr)
			}
			if emails, err := cs.Parse(); err != nil {
				t.Errorf("Send() parse output, err = %v", err)
			} else if n := len(emails); tt.wantErr && n > 0 {
				t.Errorf("Send() parse expected to return 0 on err, n = %v", n)
			} else if tt.wantErr && n == 0 {
				// happy path for error case
			} else if n != 1 {
				t.Errorf("Send() parse expected to return 1, n = %v", n)
			} else if h := emails[0].Header; !reflect.DeepEqual(h, tt.output.Headers) {
				t.Errorf("Send() header mismatch: %#v != %#v", h, tt.output.Headers)
			} else if s := emails[0].Parts[plainTextContent]; s != tt.output.PlainText {
				t.Errorf("Send() body mismatch: %q != %q", s, tt.output.PlainText)
			}
		})
	}
}
