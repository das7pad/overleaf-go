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
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/das7pad/overleaf-go/cmd/pkg/utils"
	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/integrationTests"
	"github.com/das7pad/overleaf-go/pkg/models/oneTimeToken"
	"github.com/das7pad/overleaf-go/pkg/options/listenAddress"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/client/realTime"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func TestMain(m *testing.M) {
	integrationTests.Setup(m)
}

func getURL() string {
	return fmt.Sprintf("ws://%s/socket.io", listenAddress.Parse(3026))
}

func fatalIf(tb testing.TB, err error) {
	if err != nil {
		tb.Fatalf("%s: %s", time.Now().Format(time.RFC3339Nano), err)
	}
}

func randomCredentials() (sharedTypes.Email, types.UserPassword, error) {
	email := fmt.Sprintf("%d@foo.bar", time.Now().UnixNano())
	password, err := oneTimeToken.GenerateNewToken()
	return sharedTypes.Email(email), types.UserPassword(password), err
}

func jwtFactory(tb testing.TB, ctx context.Context) func() string {
	o := types.Options{}
	o.FillFromEnv()
	rClient := utils.MustConnectRedis(ctx)
	db := utils.MustConnectPostgres(ctx)

	wm, e := web.New(&o, db, rClient, "", nil, nil)
	fatalIf(tb, e)

	return func() string {
		r := httptest.NewRequest(http.MethodTrace, "/", nil)
		r = r.WithContext(ctx)
		w := httptest.NewRecorder()
		var c *httpUtils.Context
		httpUtils.HandlerFunc(func(c2 *httpUtils.Context) {
			c = c2
		}).ServeHTTP(w, r)

		sess, err := wm.GetOrCreateSession(c)
		fatalIf(tb, err)

		defer func() {
			_ = sess.Destroy(ctx)
		}()

		email, pw, err := randomCredentials()
		fatalIf(tb, err)

		err = wm.RegisterUser(c, &types.RegisterUserRequest{
			WithSession: types.WithSession{Session: sess},
			IPAddress:   "127.0.0.1",
			Email:       email,
			Password:    pw,
		}, &types.RegisterUserResponse{})
		fatalIf(tb, err)

		res := types.CreateExampleProjectResponse{}
		err = wm.CreateExampleProject(c, &types.CreateExampleProjectRequest{
			WithSession: types.WithSession{Session: sess},
			Name:        "foo",
			Template:    "none",
		}, &res)
		fatalIf(tb, err)

		projectId := *res.ProjectId

		var jwt types.GetProjectJWTResponse
		err = wm.GetProjectJWT(c, &types.GetProjectJWTRequest{
			WithSession: types.WithSession{Session: sess},
			ProjectId:   projectId,
		}, &jwt)
		fatalIf(tb, err)

		return string(jwt)
	}
}

func connectedClient(tb testing.TB, ctx context.Context, u, bootstrap string) *realTime.Client {
	c := realTime.Client{}
	_, err := c.Connect(ctx, u, bootstrap)
	fatalIf(tb, err)
	return &c
}

func bootstrapClient(tb testing.TB, ctx context.Context, u, bootstrap string) {
	c := connectedClient(tb, ctx, u, bootstrap)
	c.Close()
}

var setupOnce sync.Once
var bootstrapSharded []string

func setup(tb testing.TB) {
	setupOnce.Do(func() {
		go main()

		ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		f := jwtFactory(tb, ctx)
		for i := 0; i < 10; i++ {
			bootstrapSharded = append(bootstrapSharded, f())
		}
	})
}

func TestBootstrap(t *testing.T) {
	setup(t)

	ctx, done := context.WithTimeout(context.Background(), time.Minute)
	defer done()

	bootstrapClient(t, ctx, getURL(), bootstrapSharded[0])
}

func benchmarkBootstrapN(b *testing.B, n int) {
	if n >= 1_000 && testing.Short() {
		b.SkipNow()
	}
	setup(b)
	ctx, done := context.WithTimeout(context.Background(), 3*time.Minute)
	defer done()

	url := getURL()

	wg := sync.WaitGroup{}
	wg.Add((n/len(bootstrapSharded) + len(bootstrapSharded)) * len(bootstrapSharded))
	for j := 0; j < n/len(bootstrapSharded)+len(bootstrapSharded); j++ {
		for _, bootstrap := range bootstrapSharded {
			go func(bootstrap string) {
				defer wg.Done()
				bootstrapClient(b, ctx, url, bootstrap)
			}(bootstrap)
		}
	}
	wg.Wait()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(n)
		for j := 0; j < n; j++ {
			go func(bootstrap string) {
				defer wg.Done()
				bootstrapClient(b, ctx, url, bootstrap)
			}(bootstrapSharded[j%len(bootstrapSharded)])
		}
		wg.Wait()
	}
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

func singleClientSetup(tb testing.TB) (*realTime.Client, func()) {
	setup(tb)
	ctx, done := context.WithTimeout(context.Background(), time.Minute)
	c := connectedClient(tb, ctx, getURL(), bootstrapSharded[0])
	return c, func() {
		c.Close()
		done()
	}
}

func TestPing(t *testing.T) {
	c, done := singleClientSetup(t)
	defer done()

	err := c.Ping()
	fatalIf(t, err)
}

func BenchmarkPing(b *testing.B) {
	c, done := singleClientSetup(b)
	defer done()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		fatalIf(b, c.Ping())
	}
}
