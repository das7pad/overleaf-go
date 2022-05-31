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

package types

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type BuildId string

func (i BuildId) Validate() error {
	return i.validate()
}

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

var anonymousSuffix = "-" + sharedTypes.UUID{}.String()

type Namespace string

func (n Namespace) IsAnonymous() bool {
	return strings.HasSuffix(string(n), anonymousSuffix)
}
