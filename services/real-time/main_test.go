// Golang port of Overleaf
// Copyright (C) 2023-2024 Jakob Ackermann <das7pad@outlook.com>
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
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/das7pad/overleaf-go/cmd/pkg/utils"
	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/integrationTests"
	"github.com/das7pad/overleaf-go/pkg/models/oneTimeToken"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/client/realTime"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

var useUnixSocket = flag.Bool("test.use-unix-socket", false, "")

func TestMain(m *testing.M) {
	integrationTests.SetupFn(m, setup)
}

func fatalIf(err error) {
	if err != nil {
		log.Panicln(err)
	}
}

func pickTransport() {
	if *useUnixSocket {
		err := os.Setenv("LISTEN_ADDRESS", realTime.UnixRunRealTime.Name)
		fatalIf(err)
		connectFn = realTime.DialUnix
	} else {
		connectFn = realTime.DialLocalhost
	}
}

func setLimits() {
	var rl syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rl); err != nil {
		panic(err)
	}
	rl.Cur = rl.Max
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rl); err != nil {
		panic(err)
	}

	if *useUnixSocket {
		p := "/proc/sys/net/core/somaxconn"
		blob, err := os.ReadFile(p)
		fatalIf(err)
		blob = bytes.TrimRight(blob, "\n")
		maxSockets, err := strconv.ParseInt(string(blob), 10, 64)
		fatalIf(err)
		if maxSockets < 20_000 {
			log.Println("Unix socket limit is likely too low")
			log.Printf(
				"Increase limit using: $ echo 20000 | sudo tee -a %s", p,
			)
		}
	}
}

func randomCredentials() (sharedTypes.Email, types.UserPassword, error) {
	email := fmt.Sprintf("%d@foo.bar", time.Now().UnixNano())
	password, err := oneTimeToken.GenerateNewToken()
	return sharedTypes.Email(email), types.UserPassword(password), err
}

func jwtFactory(ctx context.Context) func() string {
	o := types.Options{}
	o.FillFromEnv()
	rClient := utils.MustConnectRedis(ctx)
	db := utils.MustConnectPostgres(ctx)

	wm, e := web.New(&o, db, rClient, "", nil, nil)
	fatalIf(e)

	return func() string {
		r := httptest.NewRequest(http.MethodTrace, "/", nil)
		r = r.WithContext(ctx)
		w := httptest.NewRecorder()
		var c *httpUtils.Context
		httpUtils.HandlerFunc(func(c2 *httpUtils.Context) {
			c = c2
		}).ServeHTTP(w, r)

		sess, err := wm.GetOrCreateSession(c)
		fatalIf(err)

		defer func() {
			_ = sess.Destroy(ctx)
		}()

		email, pw, err := randomCredentials()
		fatalIf(err)

		err = wm.RegisterUser(c, &types.RegisterUserRequest{
			WithSession: types.WithSession{Session: sess},
			IPAddress:   "127.0.0.1",
			Email:       email,
			Password:    pw,
		}, &types.RegisterUserResponse{})
		fatalIf(err)

		res := types.CreateExampleProjectResponse{}
		err = wm.CreateExampleProject(c, &types.CreateExampleProjectRequest{
			WithSession: types.WithSession{Session: sess},
			Name:        "foo",
			Template:    "none",
		}, &res)
		fatalIf(err)

		projectId := *res.ProjectId

		var jwt types.GetProjectJWTResponse
		err = wm.GetProjectJWT(c, &types.GetProjectJWTRequest{
			WithSession: types.WithSession{Session: sess},
			ProjectId:   projectId,
		}, &jwt)
		fatalIf(err)

		return string(jwt)
	}
}

const url = "ws://127.0.0.1:3026/socket.io"

func connectedClient(bootstrap string) *realTime.Client {
	c := realTime.Client{}
	_, err := c.Connect(context.Background(), url, bootstrap, connectFn)
	if err != nil {
		fatalIf(err)
	}
	return &c
}

func bootstrapClient(bootstrap string) {
	c := connectedClient(bootstrap)
	c.Close()
}

var bootstrapSharded []string
var connectFn realTime.ConnectFn

func setup() {
	pickTransport()
	setLimits()
	go main()

	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	f := jwtFactory(ctx)
	for i := 0; i < 10; i++ {
		bootstrapSharded = append(bootstrapSharded, f())
	}
}

func TestBootstrap(t *testing.T) {
	bootstrapClient(bootstrapSharded[0])
}

func benchmarkBootstrapN(b *testing.B, n int) {
	if n >= 1_000 && testing.Short() {
		b.SkipNow()
	}

	wg := sync.WaitGroup{}
	wg.Add((n/len(bootstrapSharded) + len(bootstrapSharded)) * len(bootstrapSharded))
	for j := 0; j < n/len(bootstrapSharded)+len(bootstrapSharded); j++ {
		for _, bootstrap := range bootstrapSharded {
			go func(bootstrap string) {
				defer wg.Done()
				bootstrapClient(bootstrap)
			}(bootstrap)
		}
	}
	wg.Wait()

	b.ReportAllocs()
	b.ResetTimer()
	wg.Add(n)
	for j := 0; j < n; j++ {
		go func(bootstrap string) {
			defer wg.Done()
			for i := 0; i < b.N; i++ {
				bootstrapClient(bootstrap)
			}
		}(bootstrapSharded[j%len(bootstrapSharded)])
	}
	wg.Wait()
	b.StopTimer()
	nsPerClient := int(b.Elapsed()) / b.N / n
	b.ReportMetric(float64(int(time.Second)/nsPerClient), "clients/s")
}

func BenchmarkBootstrap1(b *testing.B) {
	benchmarkBootstrapN(b, 1)
}

func BenchmarkBootstrap10(b *testing.B) {
	benchmarkBootstrapN(b, 10)
}

func BenchmarkBootstrap100(b *testing.B) {
	benchmarkBootstrapN(b, 100)
}

func BenchmarkBootstrap200(b *testing.B) {
	benchmarkBootstrapN(b, 200)
}

func BenchmarkBootstrap500(b *testing.B) {
	benchmarkBootstrapN(b, 500)
}

func BenchmarkBootstrap1k(b *testing.B) {
	benchmarkBootstrapN(b, 1_000)
}

func BenchmarkBootstrap2k(b *testing.B) {
	benchmarkBootstrapN(b, 2_000)
}

func BenchmarkBootstrap3k(b *testing.B) {
	benchmarkBootstrapN(b, 3_000)
}

func BenchmarkBootstrap4k5(b *testing.B) {
	benchmarkBootstrapN(b, 4_500)
}

func BenchmarkBootstrap5k3(b *testing.B) {
	benchmarkBootstrapN(b, 5_300)
}

func BenchmarkBootstrap6k5(b *testing.B) {
	benchmarkBootstrapN(b, 6_500)
}

func BenchmarkBootstrap8k(b *testing.B) {
	benchmarkBootstrapN(b, 8_000)
}

func singleClientSetup() *realTime.Client {
	return connectedClient(bootstrapSharded[0])
}

func TestPing(t *testing.T) {
	c := singleClientSetup()
	defer c.Close()

	err := c.Ping()
	fatalIf(err)
}

func BenchmarkPing(b *testing.B) {
	c := singleClientSetup()
	defer c.Close()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err := c.Ping(); err != nil {
			fatalIf(err)
		}
	}
}
