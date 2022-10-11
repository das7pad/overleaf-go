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

package objectStorage

import (
	"context"
	"io"
	"net/url"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type Options struct {
	Bucket          string        `json:"bucket"`
	Provider        string        `json:"provider"`
	Endpoint        string        `json:"endpoint"`
	Region          string        `json:"region"`
	Secure          bool          `json:"secure"`
	Key             string        `json:"key"`
	Secret          string        `json:"secret"`
	SignedURLExpiry time.Duration `json:"signed_url_expiry_in_ns"`
}

func (o Options) Validate() error {
	switch o.Provider {
	case "minio":
		if o.Bucket == "" {
			return &errors.ValidationError{Msg: "missing bucket"}
		}
		if o.Endpoint == "" {
			return &errors.ValidationError{Msg: "missing endpoint"}
		}
		if o.SignedURLExpiry == 0 {
			return &errors.ValidationError{
				Msg: "missing signed_url_expiry_in_ns",
			}
		}
	default:
		return &errors.ValidationError{Msg: "unknown provider: " + o.Provider}
	}
	return nil
}

type Backend interface {
	CopyObject(ctx context.Context, src string, dest string) error
	DeleteObject(ctx context.Context, key string) error
	DeletePrefix(ctx context.Context, prefix string) error
	GetObjectSize(ctx context.Context, key string) (int64, error)
	GetReadStream(ctx context.Context, key string) (int64, io.ReadSeekCloser, error)
	GetRedirectURLForGET(ctx context.Context, key string) (*url.URL, error)
	SendFromStream(ctx context.Context, key string, reader io.Reader, size int64) error
}

func FromOptions(options Options) (Backend, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	switch options.Provider {
	case "minio":
		return initMinioBackend(options)
	default:
		return nil, errors.New("unknown provider: " + options.Provider)
	}
}
