// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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

package healthCheck

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/pendingOperation"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	SmokeTestAPI(ctx context.Context) error
	SmokeTestFull(ctx context.Context, response *types.SmokeTestResponse) error
}

func New(options *types.Options, client redis.UniversalClient, um user.Manager, localURL string) (Manager, error) {
	loginBody, err := json.Marshal(types.LoginRequest{
		Email:    options.SmokeTest.Email,
		Password: options.SmokeTest.Password,
	})
	if err != nil {
		return nil, errors.Tag(err, "marshal login body")
	}
	projectIdMeta, err := regexp.Compile(
		"<meta\\s+name=\"ol-project_id\"\\s+data-type=\"string\"\\s+content=\"" + options.SmokeTest.ProjectId.String() + "\"\\s*/>",
	)
	if err != nil {
		return nil, errors.Tag(err, "compile projectId meta regex")
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, errors.Tag(err, "get hostname")
	}
	rawRand := make([]byte, 4)
	if _, err = rand.Read(rawRand); err != nil {
		return nil, errors.Tag(err, "get random blob")
	}
	rnd := hex.EncodeToString(rawRand)

	return &manager{
		client: client,
		um:     um,
		randomPrefix: fmt.Sprintf(
			"%s:%s:%d", hostname, rnd, os.Getpid(),
		),

		smokeTestLoginBody:     loginBody,
		smokeTestProjectIdMeta: projectIdMeta,
		smokeTestProjectIdHex:  options.SmokeTest.ProjectId.String(),
		smokeTestBaseURL:       localURL,
		smokeTestUserId:        options.SmokeTest.UserId,
	}, nil
}

type manager struct {
	client       redis.UniversalClient
	randomPrefix string
	um           user.Manager

	apiMux        sync.Mutex
	apiPending    pendingOperation.PendingOperation
	apiValidUntil time.Time

	smokeTestLoginBody     []byte
	smokeTestProjectIdMeta *regexp.Regexp
	smokeTestProjectIdHex  string
	smokeTestBaseURL       string
	smokeTestUserId        sharedTypes.UUID
}
