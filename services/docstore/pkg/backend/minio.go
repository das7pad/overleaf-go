// Golang port of the Overleaf docstore service
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

package backend

import (
	"context"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
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
		return ErrorNotFound{}
	}
	return err
}

func (m *minioBackend) SendFromFile(ctx context.Context, bucket string, key string, filePath string, options SendOptions) error {
	_, err := m.mc.FPutObject(ctx, bucket, key, filePath, minio.PutObjectOptions{
		SendContentMd5:  true,
		ContentEncoding: options.ContentEncoding,
		ContentType:     options.ContentType,
	})
	return err
}

func (m *minioBackend) SendFromStream(ctx context.Context, bucket string, key string, reader io.Reader, options SendOptions) error {
	_, err := m.mc.PutObject(ctx, bucket, key, reader, options.ContentSize, minio.PutObjectOptions{
		ContentType:     options.ContentType,
		ContentEncoding: options.ContentEncoding,
		SendContentMd5:  true,
	})
	return err
}

func (m *minioBackend) GetReadStream(ctx context.Context, bucket string, key string, options GetOptions) (io.Reader, error) {
	opts := minio.GetObjectOptions{}
	if options.Start != 0 || options.End != 0 {
		if err := opts.SetRange(options.Start, options.End); err != nil {
			return nil, err
		}
	}

	r, err := m.mc.GetObject(ctx, bucket, key, opts)
	if err != nil {
		return nil, rewriteError(err)
	}
	return r, nil
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

func (m *minioBackend) GetRedirectURLForHEAD(ctx context.Context, bucket string, key string) (*url.URL, error) {
	return m.mc.PresignedHeadObject(
		ctx,
		bucket,
		key,
		m.signedURLExpiry,
		nil,
	)
}

func (m *minioBackend) GetRedirectURLForPOST(ctx context.Context, bucket string, key string) (*url.URL, FormData, error) {
	policy := minio.NewPostPolicy()
	if err := policy.SetBucket(bucket); err != nil {
		return nil, nil, err
	}
	if err := policy.SetKey(key); err != nil {
		return nil, nil, err
	}
	err := policy.SetExpires(time.Now().UTC().Add(m.signedURLExpiry))
	if err != nil {
		return nil, nil, err
	}

	return m.mc.PresignedPostPolicy(ctx, policy)
}

func (m *minioBackend) GetRedirectURLForPUT(ctx context.Context, bucket string, key string) (*url.URL, error) {
	return m.mc.PresignedPutObject(
		ctx,
		bucket,
		key,
		m.signedURLExpiry,
	)
}

func (m *minioBackend) GetObjectSize(ctx context.Context, bucket string, key string) (int64, error) {
	o, err := m.mc.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return 0, rewriteError(err)
	}
	return o.Size, nil
}

func (m *minioBackend) GetDirectorySize(ctx context.Context, bucket string, prefix string) (int64, error) {
	c := m.mc.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		Prefix: prefix,
	})
	var sum int64
	for info := range c {
		if err := info.Err; err != nil {
			return 0, rewriteError(err)
		}
		sum += info.Size
	}
	return sum, nil
}

func (m *minioBackend) DeleteObject(ctx context.Context, bucket string, key string) error {
	return m.mc.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
}

func (m *minioBackend) DeletePrefix(ctx context.Context, bucket string, prefix string) error {
	objects := m.mc.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		Prefix: prefix,
	})
	errors := m.mc.RemoveObjects(ctx, bucket, objects, minio.RemoveObjectsOptions{})

	for objectError := range errors {
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
