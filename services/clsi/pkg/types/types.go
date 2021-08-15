// Golang port of the Overleaf clsi service
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

package types

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/services/clsi/pkg/errors"
)

type BuildId string

func (i BuildId) IsStringParameter() bool {
	return true
}

func (i BuildId) Validate(_ *Options) error {
	return i.validate()
}

const allZeroBuildId = BuildId("0000000000000000-0000000000000000")

var buildIdRegex = regexp.MustCompile("^[a-f0-9]{16}-[a-f0-9]{16}$")

func (i BuildId) validate() error {
	if i == "" {
		return &errors.ValidationError{Msg: "buildId missing"}
	}
	if len(i) != 33 {
		return &errors.ValidationError{Msg: "buildId not 33 char long"}
	}
	if !buildIdRegex.MatchString(string(i)) {
		return &errors.ValidationError{Msg: "buildId malformed"}
	}
	return nil
}

func (i BuildId) Age() (time.Duration, error) {
	if err := i.validate(); err != nil {
		return 0, err
	}
	ns, err := strconv.ParseInt(string(i)[:16], 16, 64)
	if err != nil {
		return 0, err
	}
	return time.Now().Sub(time.Unix(0, ns)), nil
}

type CompileGroup string

func (c CompileGroup) IsStringParameter() bool {
	return true
}

func (c CompileGroup) Validate(options *Options) error {
	if c == "" {
		return &errors.ValidationError{Msg: "compileGroup missing"}
	}
	if len(options.AllowedCompileGroups) == 0 {
		return nil
	}
	for _, compileGroup := range options.AllowedCompileGroups {
		if c == compileGroup {
			return nil
		}
	}
	return &errors.ValidationError{Msg: "compileGroup is not allowed"}
}

var imageNameYearRegex = regexp.MustCompile(":([0-9]+)\\.[0-9]+")

type ImageName string

func (i ImageName) Year() string {
	m := imageNameYearRegex.FindStringSubmatch(string(i))
	return m[1]
}

func (i ImageName) IsStringParameter() bool {
	return true
}

func (i ImageName) Validate(options *Options) error {
	if i == "" {
		return &errors.ValidationError{
			Msg: "imageName missing",
		}
	}
	if !imageNameYearRegex.MatchString(string(i)) {
		return &errors.ValidationError{
			Msg: "imageName does not match year regex",
		}
	}
	if len(options.AllowedImages) == 0 {
		return nil
	}
	for _, image := range options.AllowedImages {
		if i == image {
			return nil
		}
	}
	return &errors.ValidationError{Msg: "imageName is not allowed"}
}

var anonymousSuffix = "-" + primitive.NilObjectID.Hex()

type Namespace string

func (n Namespace) IsAnonymous() bool {
	return strings.HasSuffix(string(n), anonymousSuffix)
}
