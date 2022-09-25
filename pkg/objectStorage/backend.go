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
	Provider        string        `json:"provider"`
	Endpoint        string        `json:"endpoint"`
	Secure          bool          `json:"secure"`
	Key             string        `json:"key"`
	Secret          string        `json:"secret"`
	SignedURLExpiry time.Duration `json:"signed_url_expiry_in_ns"`
}

type GetOptions struct {
	Start int64
	End   int64
}

type SendOptions struct {
	ContentSize int64
}

type Backend interface {
	SendFromStream(
		ctx context.Context,
		bucket string,
		key string,
		reader io.Reader,
		options SendOptions,
	) error

	GetReadStream(
		ctx context.Context,
		bucket string,
		key string,
		options GetOptions,
	) (int64, io.ReadCloser, error)

	GetRedirectURLForGET(
		ctx context.Context,
		bucket string,
		key string,
	) (*url.URL, error)

	GetObjectSize(
		ctx context.Context,
		bucket string,
		key string,
	) (int64, error)

	DeleteObject(
		ctx context.Context,
		bucket string,
		key string,
	) error

	DeletePrefix(
		ctx context.Context,
		bucket string,
		prefix string,
	) error

	CopyObject(
		ctx context.Context,
		bucket string,
		src string,
		dest string,
	) error
}

func FromOptions(options Options) (Backend, error) {
	switch options.Provider {
	case "minio":
		return initMinioBackend(options)
	default:
		return nil, errors.New("unknown provider: " + options.Provider)
	}
}
