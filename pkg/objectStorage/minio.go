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

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

func initMinioBackend(o Options) (Backend, error) {
	mc, err := minio.New(o.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(o.Key, o.Secret, ""),
		Secure: o.Secure,
	})
	if err != nil {
		return nil, err
	}
	return &minioBackend{
		mc:              mc,
		signedURLExpiry: o.SignedURLExpiry,
	}, nil
}

type minioBackend struct {
	mc              *minio.Client
	signedURLExpiry time.Duration
}

func rewriteError(err error) error {
	if err == nil {
		return nil
	}
	minioError, isMinioError := err.(minio.ErrorResponse)
	if isMinioError && minioError.Code == "NoSuchKey" {
		return &errors.NotFoundError{}
	}
	return err
}

func (m *minioBackend) SendFromStream(ctx context.Context, bucket string, key string, reader io.Reader, options SendOptions) error {
	_, err := m.mc.PutObject(ctx, bucket, key, reader, options.ContentSize, minio.PutObjectOptions{
		SendContentMd5: true,
	})
	return err
}

func (m *minioBackend) GetReadStream(ctx context.Context, bucket string, key string) (int64, io.ReadCloser, error) {
	r, err := m.mc.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return 0, nil, errors.Tag(rewriteError(err), "get")
	}
	// We need to peek into the s3.GetObject response.
	// This _saves_ one s3.HeadObject request for the size.
	_, err = r.Read(make([]byte, 0))
	if err == io.EOF {
		if s, err2 := r.Stat(); err2 == nil && s.Size == 0 {
			_ = r.Close()
			// This is an empty file.
			return 0, r, nil
		}
	}
	if err != nil {
		_ = r.Close()
		return 0, nil, errors.Tag(rewriteError(err), "probe")
	}
	s, err := r.Stat()
	if err != nil {
		_ = r.Close()
		return 0, nil, errors.Tag(rewriteError(err), "stat")
	}
	return s.Size, r, nil
}

func (m *minioBackend) GetRedirectURLForGET(ctx context.Context, bucket string, key string) (*url.URL, error) {
	params := make(url.Values)
	params.Set("Response-Content-Disposition", "attachment")
	params.Set("Response-Content-Type", "application/octet-stream")
	return m.mc.PresignedGetObject(
		ctx,
		bucket,
		key,
		m.signedURLExpiry,
		params,
	)
}

func (m *minioBackend) GetObjectSize(ctx context.Context, bucket string, key string) (int64, error) {
	o, err := m.mc.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return 0, rewriteError(err)
	}
	return o.Size, nil
}

func (m *minioBackend) DeleteObject(ctx context.Context, bucket string, key string) error {
	err := m.mc.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
	return rewriteError(err)
}

func (m *minioBackend) DeletePrefix(ctx context.Context, bucket string, prefix string) error {
	objects := m.mc.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		Prefix: prefix,
	})
	objectErrors := m.mc.RemoveObjects(ctx, bucket, objects, minio.RemoveObjectsOptions{})

	for objectError := range objectErrors {
		return rewriteError(objectError.Err)
	}
	return nil
}

func (m *minioBackend) CopyObject(ctx context.Context, bucket string, src string, dest string) error {
	_, err := m.mc.CopyObject(
		ctx,
		minio.CopyDestOptions{
			Bucket: bucket,
			Object: dest,
		},
		minio.CopySrcOptions{
			Bucket: bucket,
			Object: src,
		},
	)
	return rewriteError(err)
}
