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

package main

import (
	"context"
	"os"

	"github.com/das7pad/overleaf-go/cmd/minio-setup/pkg/minio-setup"
)

func main() {
	o := minioSetup.Options{
		Endpoint:         os.Getenv("MINIO_ENDPOINT"),
		Secure:           os.Getenv("MINIO_SECURE") == "true",
		Region:           os.Getenv("MINIO_REGION"),
		RootUser:         os.Getenv("MINIO_ROOT_USER"),
		RootPassword:     os.Getenv("MINIO_ROOT_PASSWORD"),
		Bucket:           os.Getenv("BUCKET"),
		AccessKey:        os.Getenv("ACCESS_KEY"),
		SecretKey:        os.Getenv("SECRET_KEY"),
		PolicyName:       os.Getenv("S3_POLICY_NAME"),
		PolicyContent:    os.Getenv("S3_POLICY_CONTENT"),
		CleanupOtherKeys: os.Getenv("CLEANUP_OTHER_S3_KEYS") == "true",
	}
	err := minioSetup.Setup(context.Background(), o)
	if err != nil {
		panic(err)
	}
}
