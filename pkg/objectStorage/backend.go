// Golang port of Overleaf
// Copyright (C) 2021 Jakob Ackermann <das7pad@outlook.com>
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
	"fmt"
	"io"
	"net/url"
	"time"
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
	ContentSize     int64
	ContentType     string
	ContentEncoding string
}

type FormData map[string]string

type Backend interface {
	SendFromFile(
		ctx context.Context,
		bucket string,
		key string,
		filePath string,
		options SendOptions,
	) error

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
	) (io.Reader, error)

	GetRedirectURLForGET(
		ctx context.Context,
		bucket string,
		key string,
	) (*url.URL, error)

	GetRedirectURLForHEAD(
		ctx context.Context,
		bucket string,
		key string,
	) (*url.URL, error)

	GetRedirectURLForPOST(
		ctx context.Context,
		bucket string,
		key string,
	) (*url.URL, FormData, error)

	GetRedirectURLForPUT(
		ctx context.Context,
		bucket string,
		key string,
	) (*url.URL, error)

	GetObjectSize(
		ctx context.Context,
		bucket string,
		key string,
	) (int64, error)

	GetDirectorySize(
		ctx context.Context,
		bucket string,
		prefix string,
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
	}
	return nil, fmt.Errorf("unknown provider: %s", options.Provider)
}
