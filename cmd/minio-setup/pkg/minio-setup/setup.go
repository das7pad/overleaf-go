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

package minioSetup

import (
	"context"
	"log"
	"time"

	"github.com/minio/madmin-go/v2"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type Options struct {
	Endpoint         string
	Secure           bool
	Region           string
	RootUser         string
	RootPassword     string
	Bucket           string
	AccessKey        string
	SecretKey        string
	PolicyName       string
	PolicyContent    string
	CleanupOtherKeys bool
}

func Setup(ctx context.Context, o Options) error {
	mc, err := minio.New(o.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(o.RootUser, o.RootPassword, ""),
		Region: o.Region,
		Secure: o.Secure,
	})
	if err != nil {
		return errors.Tag(err, "create mc client")
	}

	for i := 0; i < 10; i++ {
		if _, err = mc.BucketExists(ctx, o.Bucket); err != nil {
			log.Printf("minio not ready: %s", err)
			time.Sleep(time.Second)
			continue
		}
		break
	}

	log.Println("Creating bucket")
	err = mc.MakeBucket(ctx, o.Bucket, minio.MakeBucketOptions{
		Region: o.Region,
	})
	if err != nil &&
		minio.ToErrorResponse(err).Code != "BucketAlreadyExists" &&
		minio.ToErrorResponse(err).Code != "BucketAlreadyOwnedByYou" {
		return errors.Tag(err, "create bucket")
	}

	c, err := madmin.New(o.Endpoint, o.RootUser, o.RootPassword, o.Secure)
	if err != nil {
		return errors.Tag(err, "create admin client")
	}
	if o.CleanupOtherKeys {
		log.Println("Listing other users")
		users, err2 := c.ListUsers(ctx)
		if err2 != nil {
			return errors.Tag(err2, "list other users")
		}
		for s := range users {
			if s == o.AccessKey {
				continue
			}
			log.Printf("Removing other user %s", s)
			if err2 = c.RemoveUser(ctx, s); err2 != nil {
				return errors.Tag(err2, "remove other user "+s)
			}
		}
	}

	log.Println("Creating user")
	if err = c.AddUser(ctx, o.AccessKey, o.SecretKey); err != nil {
		return errors.Tag(err, "create user")
	}

	log.Println("Adding policy")
	err = c.AddCannedPolicy(ctx, o.PolicyName, []byte(o.PolicyContent))
	if err != nil {
		return errors.Tag(err, "add policy")
	}

	log.Println("Setting policy")
	err = c.SetPolicy(ctx, o.PolicyName, o.AccessKey, false)
	if err != nil {
		return errors.Tag(err, "set policy")
	}

	log.Println("Done.")
	return nil
}
