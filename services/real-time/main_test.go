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
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web"
	webTypes "github.com/das7pad/overleaf-go/services/web/pkg/types"
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
		if maxSockets < 30_000 {
			log.Println("Unix socket limit is likely too low")
			log.Printf(
				"Increase limit using: $ echo 30000 | sudo tee -a %s", p,
			)
		}
	}
}

func randomCredentials() (sharedTypes.Email, webTypes.UserPassword, error) {
	email := fmt.Sprintf("%d@foo.bar", time.Now().UnixNano())
	password, err := oneTimeToken.GenerateNewToken()
	return sharedTypes.Email(email), webTypes.UserPassword(password), err
}

func jwtFactory(ctx context.Context) func() string {
	o := webTypes.Options{}
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

		err = wm.RegisterUser(c, &webTypes.RegisterUserRequest{
			WithSession: webTypes.WithSession{Session: sess},
			IPAddress:   "127.0.0.1",
			Email:       email,
			Password:    pw,
		}, &webTypes.RegisterUserResponse{})
		fatalIf(err)

		res := webTypes.CreateExampleProjectResponse{}
		err = wm.CreateExampleProject(c, &webTypes.CreateExampleProjectRequest{
			WithSession: webTypes.WithSession{Session: sess},
			Name:        "foo",
			Template:    "none",
		}, &res)
		fatalIf(err)

		projectId := *res.ProjectId

		var jwt webTypes.GetProjectJWTResponse
		err = wm.GetProjectJWT(c, &webTypes.GetProjectJWTRequest{
			WithSession: webTypes.WithSession{Session: sess},
			ProjectId:   projectId,
		}, &jwt)
		fatalIf(err)

		return string(jwt)
	}
}

var uri = &url.URL{
	Scheme: "ws",
	Host:   "127.0.0.1:3026",
	Path:   "/socket.io",
}

func connectedClient(bootstrap string) (*types.RPCResponse, *realTime.Client) {
	c := realTime.Client{}
	res, err := c.Connect(context.Background(), uri, bootstrap, connectFn)
	if err != nil {
		fatalIf(err)
	}
	return res, &c
}

func bootstrapClient(bootstrap string) {
	_, c := connectedClient(bootstrap)
	c.Close()
}

func bootstrapClientN(bootstrap string, n int, wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; i < n; i++ {
		bootstrapClient(bootstrap)
	}
}

var bootstrapSharded []string
var connectFn realTime.ConnectFn

func setup() {
	pickTransport()
	setLimits()
	go main()

	f := jwtFactory(context.Background())
	for i := 0; i < 10; i++ {
		bootstrapSharded = append(bootstrapSharded, f())
	}
}

func TestStatus(t *testing.T) {
	tr := http.Transport{}
	tr.DialContext = connectFn
	d := http.Client{
		Transport: &tr,
	}
	r, err := d.Get("http://127.0.0.1:3026/status")
	fatalIf(err)

	if r.StatusCode != http.StatusOK {
		t.Fatalf("status code != 200: %d", r.StatusCode)
	}
}

func TestBootstrap(t *testing.T) {
	bootstrapClient(bootstrapSharded[0])
}

func benchmarkBootstrapN(b *testing.B, n int) {
	if n >= 1_000 && testing.Short() {
		b.SkipNow()
	}

	wg := &sync.WaitGroup{}
	wg.Add((n/len(bootstrapSharded) + len(bootstrapSharded)) * len(bootstrapSharded))
	for j := 0; j < n/len(bootstrapSharded)+len(bootstrapSharded); j++ {
		for _, bootstrap := range bootstrapSharded {
			go bootstrapClientN(bootstrap, 3, wg)
		}
	}
	wg.Wait()

	b.ReportAllocs()
	b.ResetTimer()
	wg.Add(n)
	for j := 0; j < n; j++ {
		go bootstrapClientN(bootstrapSharded[j%len(bootstrapSharded)], b.N, wg)
	}
	wg.Wait()
	b.StopTimer()
	nsPerClient := int(b.Elapsed()) / b.N / n
	b.ReportMetric(float64(int(time.Second)/nsPerClient), "clients/s")

	if b.N > 1 {
		// wait for unsubscribe
		time.Sleep(500 * time.Millisecond)
	}
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

func BenchmarkBootstrap9k(b *testing.B) {
	benchmarkBootstrapN(b, 9_000)
}

func BenchmarkBootstrap14k(b *testing.B) {
	benchmarkBootstrapN(b, 14_000)
}

func BenchmarkBootstrap15k(b *testing.B) {
	benchmarkBootstrapN(b, 15_000)
}

func BenchmarkBootstrap16k(b *testing.B) {
	benchmarkBootstrapN(b, 16_000)
}

func BenchmarkBootstrap17k(b *testing.B) {
	benchmarkBootstrapN(b, 17_000)
}

func BenchmarkBootstrap20k(b *testing.B) {
	benchmarkBootstrapN(b, 20_000)
}

func BenchmarkBootstrap21k(b *testing.B) {
	benchmarkBootstrapN(b, 21_000)
}

func singleClientSetup() (*types.RPCResponse, *realTime.Client) {
	return connectedClient(bootstrapSharded[0])
}

func TestPing(t *testing.T) {
	_, c := singleClientSetup()
	defer c.Close()

	err := c.Ping()
	fatalIf(err)
}

func BenchmarkPing(b *testing.B) {
	_, c := singleClientSetup()
	defer c.Close()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err := c.Ping(); err != nil {
			fatalIf(err)
		}
	}
}

func TestConnectedClients(t *testing.T) {
	res, c := singleClientSetup()
	defer c.Close()

	var bs types.BootstrapWSResponse
	if err := json.Unmarshal(res.Body, &bs); err != nil {
		t.Fatalf("deserialize bootstrap response: %q: %s", string(res.Body), err)
	}
	id1 := bs.PublicId

	var cc types.ConnectedClients
	if err := json.Unmarshal(bs.ConnectedClients, &cc); err != nil {
		t.Fatalf("deserialize connected clients: %q: %s", string(bs.ConnectedClients), err)
	}
	var seen []sharedTypes.PublicId
	for _, client := range cc {
		seen = append(seen, client.ClientId)
	}
	var removed []sharedTypes.PublicId
	c.On(sharedTypes.ClientTrackingBatch, func(res types.RPCResponse) {
		var rcs types.RoomChanges
		if err := json.Unmarshal(res.Body, &rcs); err != nil {
			t.Fatalf("deserialize room changes: %q: %s", string(res.Body), err)
		}
		for _, rc := range rcs {
			if rc.IsJoin {
				seen = append(seen, rc.PublicId)
			} else {
				removed = append(removed, rc.PublicId)
			}
		}
	})

	res, o := singleClientSetup()
	if err := json.Unmarshal(res.Body, &bs); err != nil {
		t.Fatalf("deserialize 2nd bootstrap response: %q: %s", string(res.Body), err)
	}
	defer o.Close()
	id2 := bs.PublicId

	if err := c.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set deadline: %s", err)
	}

	var foundId1, foundId2 bool
	for i := 0; i < 3; i++ {
		if err := c.ReadOnce(); err != nil {
			t.Fatal(err)
		}
		for _, id := range seen {
			if id == id1 {
				foundId1 = true
			}
			if id == id2 {
				foundId2 = true
			}
		}
		if foundId1 && foundId2 {
			break
		}
	}

	if !foundId1 {
		t.Error("id1 not found")
	}
	if !foundId2 {
		t.Error("id2 not found")
	}
	if !foundId1 || !foundId2 {
		t.FailNow()
	}

	var id2Removed bool
	for _, id := range removed {
		if id == id2 {
			id2Removed = true
		}
	}
	if id2Removed {
		t.Fatal("id2 removed before disconnect")
	}

	o.Close()

	for i := 0; i < 3; i++ {
		if err := c.ReadOnce(); err != nil {
			t.Fatal(err)
		}
		for _, id := range removed {
			if id == id2 {
				id2Removed = true
			}
		}
		if id2Removed {
			break
		}
	}
	if !id2Removed {
		t.Fatal("id2 not removed after disconnect")
	}
}
