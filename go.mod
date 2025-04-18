module github.com/das7pad/overleaf-go

go 1.24

require (
	github.com/docker/docker v28.0.4+incompatible
	github.com/docker/go-connections v0.5.0
	github.com/docker/go-units v0.5.0
	github.com/evanw/esbuild v0.25.2
	github.com/gorilla/mux v1.8.1
	github.com/gorilla/websocket v1.5.1
	github.com/hashicorp/golang-lru/v2 v2.0.7
	github.com/jackc/pgx/v5 v5.7.4
	github.com/minio/madmin-go/v2 v2.2.1
	github.com/minio/minio-go/v7 v7.0.90
	github.com/moby/term v0.5.2
	github.com/redis/go-redis/v9 v9.7.3
	github.com/sergi/go-diff v1.3.1
	go.mongodb.org/mongo-driver v1.17.3
	golang.org/x/crypto v0.37.0
	golang.org/x/net v0.39.0
	golang.org/x/sync v0.13.0
)

tool golang.org/x/tools/cmd/stringer

require (
	github.com/Azure/go-ansiterm v0.0.0-20250102033503-faa5f7b0171c // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-ini/ini v1.67.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.26.3 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.10 // indirect
	github.com/lufia/plan9stats v0.0.0-20250317134145-8bc96cf8fc35 // indirect
	github.com/minio/crc64nvme v1.0.1 // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/montanaflynn/stats v0.7.1 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/philhofer/fwd v1.1.3-0.20240916144458-20a13a1f6b7c // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/prometheus/procfs v0.16.0 // indirect
	github.com/rs/xid v1.6.0 // indirect
	github.com/secure-io/sio-go v0.3.1 // indirect
	github.com/shirou/gopsutil/v3 v3.24.5 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/tinylib/msgp v1.2.5 // indirect
	github.com/tklauser/go-sysconf v0.3.15 // indirect
	github.com/tklauser/numcpus v0.10.0 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.2 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.60.0 // indirect
	go.opentelemetry.io/otel v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.35.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	golang.org/x/mod v0.24.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	golang.org/x/time v0.11.0 // indirect
	golang.org/x/tools v0.32.0 // indirect
	google.golang.org/grpc v1.71.1 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gotest.tools/v3 v3.5.2 // indirect
)

// Cut down allocations
replace github.com/gorilla/mux v1.8.1 => github.com/gorilla/mux v1.8.2-0.20231207045853-e44017df2b87

// Add LeanConn and optimize allocations
replace github.com/gorilla/websocket v1.5.1 => github.com/das7pad/gorilla-websocket v0.0.0-20240310181424-0427669d163f

// Optimize allocations and provide new interfaces
replace github.com/redis/go-redis/v9 v9.7.3 => github.com/das7pad/go-redis/v9 v9.0.0-20250416215732-f7d6c3a32276

// Optimize allocations
replace github.com/jackc/pgx/v5 v5.7.4 => github.com/das7pad/pgx/v5 v5.0.0-20250416220222-5006f5038498

// Optimize allocations
replace github.com/jackc/puddle/v2 v2.2.2 => github.com/das7pad/puddle/v2 v2.0.0-20241027234030-a63b1e47fedb
