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

package csp

import (
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type csp struct {
	reportURL        *sharedTypes.URL
	siteOrigin       string
	omitProtocol     bool
	reportViolations bool
	trimPolicy       bool

	baseURI        []string
	childSrc       []string
	connectSRC     []string
	defaultSrc     []string
	fontSrc        []string
	formAction     []string
	frameAncestors []string
	frameSrc       []string
	imgSrc         []string
	manifestSrc    []string
	mediaSrc       []string
	scriptSrc      []string
	styleSrc       []string
	workerSrc      []string
}

type directive struct {
	name         string
	isStandalone bool
	items        []string
}

func (c csp) render() string {
	// Backwards compatibility for CSP level 3 directives
	c.childSrc = append(c.childSrc, c.workerSrc...)

	directives := []directive{
		{
			name:  "base-uri",
			items: c.baseURI,
		},
		{
			name:  "child-src",
			items: c.childSrc,
		},
		{
			name:  "connect-src",
			items: c.connectSRC,
		},
		{
			name:  "default-src",
			items: c.defaultSrc,
		},
		{
			name:  "font-src",
			items: c.fontSrc,
		},
		{
			name:  "form-action",
			items: c.formAction,
		},
		{
			name:  "frame-ancestors",
			items: c.frameAncestors,
		},
		{
			name:  "frame-src",
			items: c.frameSrc,
		},
		{
			name:  "img-src",
			items: c.imgSrc,
		},
		{
			name:  "manifest-src",
			items: c.manifestSrc,
		},
		{
			name:  "media-src",
			items: c.mediaSrc,
		},
		{
			name:  "script-src",
			items: c.scriptSrc,
		},
		{
			name:  "style-src",
			items: c.styleSrc,
		},
		{
			name:  "worker-src",
			items: c.workerSrc,
		},
	}

	if c.omitProtocol {
		for _, d := range directives {
			for _, item := range d.items {
				if strings.HasPrefix(item, "http://") {
					c.omitProtocol = false
				}
			}
		}
	}
	if c.omitProtocol {
		directives = append(directives, directive{
			name:         "block-all-mixed-content",
			isStandalone: true,
		})
	}
	if c.reportViolations && c.reportURL != nil {
		directives = append(directives, directive{
			name:  "report-uri",
			items: []string{c.reportURL.String()},
		})
	}

	b := &strings.Builder{}
	for i, d := range directives {
		uniq := make(map[string]bool, 0)
		for _, item := range d.items {
			if item == "" {
				continue
			}
			if item == c.siteOrigin {
				item = "'self'"
			}
			if c.omitProtocol && d.name != "report-uri" {
				switch item {
				case "data:", "blob:", "'self'":
					// constants
				default:
					item = strings.TrimPrefix(item, "https://")
				}
			}
			uniq[item] = true
		}
		if len(uniq) == 0 {
			if c.trimPolicy &&
				d.name != "default-src" &&
				strings.HasSuffix(d.name, "-src") {
				continue
			}
			uniq["'none'"] = true
		}
		flat := make([]string, 0, len(uniq))
		for s := range uniq {
			flat = append(flat, s)
		}
		sort.Slice(flat, func(i, j int) bool {
			return flat[i] < flat[j]
		})

		if i != 0 {
			b.WriteString("; ")
		}
		b.WriteString(d.name)
		if d.isStandalone {
			continue
		}
		for _, s := range flat {
			b.WriteString(" ")
			b.WriteString(s)
		}
	}
	return b.String()
}

func getDigest(blob string) string {
	h := sha512.New()
	h.Write([]byte(blob))
	return fmt.Sprintf(
		"'sha512-%s'",
		base64.StdEncoding.EncodeToString(h.Sum(nil)),
	)
}

type Options struct {
	CDNURL            sharedTypes.URL
	PdfDownloadDomain *sharedTypes.URL
	ReportURL         *sharedTypes.URL
	SentryDSN         *sharedTypes.URL
	SiteURL           sharedTypes.URL
}

type CSPs struct {
	API       string
	Angular   string
	Marketing string
	NoJs      string
	Editor    string
	Learn     string
}

func getOrigin(u *sharedTypes.URL) string {
	if u == nil {
		return ""
	}
	return (&url.URL{
		Scheme: u.Scheme,
		Host:   u.Host,
	}).String()
}

func Generate(o Options) CSPs {
	siteOrigin := getOrigin(&o.SiteURL)
	cdnOrigin := getOrigin(&o.CDNURL)
	pdfDownloadOrigin := getOrigin(o.PdfDownloadDomain)
	sentryOrigin := getOrigin(o.SentryDSN)

	noJs := csp{
		omitProtocol:     true,
		reportURL:        o.ReportURL,
		reportViolations: true,
		siteOrigin:       siteOrigin,

		baseURI:    []string{siteOrigin},
		connectSRC: []string{},
		fontSrc:    []string{cdnOrigin},
		formAction: []string{siteOrigin},
		frameSrc:   []string{},
		imgSrc:     []string{cdnOrigin, "data:", "blob:"},
		scriptSrc:  []string{},
		styleSrc:   []string{cdnOrigin},
	}

	js := noJs
	js.connectSRC = append(js.connectSRC, siteOrigin, sentryOrigin)
	js.scriptSrc = append(js.scriptSrc, cdnOrigin)

	angular := js
	// angular-sanitize probe
	angular.styleSrc = append(angular.styleSrc, getDigest(`<img src="`))

	marketing := js

	learn := marketing
	// Media files and Tutorial videos.
	learn.frameSrc = append(learn.frameSrc, "https://www.youtube.com")
	learn.imgSrc = append(learn.imgSrc, siteOrigin, "https://images.ctfassets.net", "https://wikimedia.org", "https://www.filepicker.io")
	learn.mediaSrc = append(learn.mediaSrc, "https://videos.ctfassets.net")
	// Too many inline styles to put on an allow-list.
	learn.styleSrc = append(learn.styleSrc, "'unsafe-inline'")

	editor := angular
	// PDF.js character maps and PDF/logs requests
	editor.connectSRC = append(editor.connectSRC, cdnOrigin, pdfDownloadOrigin)
	// Browser pdf viewer
	editor.frameSrc = append(editor.frameSrc, pdfDownloadOrigin)
	// Binary file preview
	editor.imgSrc = append(editor.imgSrc, siteOrigin)
	// Ace and pdf.js
	editor.workerSrc = append(editor.workerSrc, "blob:")
	if siteOrigin == cdnOrigin {
		// PDF.js worker loading supports CORS and has an 'optimization'.
		// - siteOrigin!=cdnOrigin: Load Worker from Blob with importScripts().
		// - siteOrigin==cdnOrigin: Load Worker directly from URL.
		editor.workerSrc = append(editor.workerSrc, siteOrigin)
	}

	api := csp{
		siteOrigin: siteOrigin,
		trimPolicy: true,
		imgSrc:     []string{siteOrigin, cdnOrigin},
	}

	return CSPs{
		API:       api.render(),
		Angular:   angular.render(),
		Marketing: marketing.render(),
		NoJs:      noJs.render(),
		Editor:    editor.render(),
		Learn:     learn.render(),
	}
}
