# Overleaf-Go

Welcome to the Golang port of [Overleaf](https://github.com/overleaf/overleaf)!

This repository is a monorepo for all the backend services and other useful
command line utils.
The frontend code remains in a [fork of the Node.js monorepo](
https://github.com/das7pad/overleaf-node).

## Highlights compared to the original Node.js implementation

- The backend code is written in a statically typed language
- Monolith-first approach, eliminating latency overhead and network chatter --
  just run this single binary, and you are good (compiles can still be scaled
  horizontally, other components as well - but they are self-contained and
  do not expose internal APIs on the network)
- Mongodb was replaced with PostgreSQL (an automated data migration exists via
  `cmd/m2pq`, you can find a mongodb implementation and edgedb implementation
  in the git history too)
- Postgres enables single round-trip data fetching for complex pages like the
  project dashboard or the project editor page
- Postgres enables atomic updates to the filetree and strong unique constrains
  inside the tree
- No admin capabilities in the web app (command line utils exist instead)
- Highly optimized compile process (<200ms are possible for incremental
  compiles, <20ms for synctex aka navigating between code/PDF location, zero
  content is fetched from Postgres for incremental compiles)
- Homegrown real-time service with minimal overhead for connection startup (
  zero server round trips inside the websocket for "joining" a project)
- JWT based authentication and authorization for many endpoints (epochs in the
  DB enable invalidation of tokens ahead of their expiry)
- Highly optimized document updating process (merged redis keys reduce redis
  calls by an order of magnitude, re-using of already fetched content for
  processing multiple updates and flushing them all the way to history/DB)
- Worker-stealing for spellchecker (this allows less popular languages to
  take over idle worker from a more popular language)
- Chaining of network proxies for fetching external resources (multiple hops
  between proxies allow stronger firewalling of services, e.g. hop to local
  service and then hop to external cloud -- only that local service needs
  internet access)
- Stronger CSP and zero inline scripts in HTML templates
- Minify and compile HTML templates at boot-time in well under 1s
- Caching for documentation content and images
- Website translations on a single domain
- No local filesystem backend for storing binary files (all major provider
  expose an S3 compatible API today and min.io can be used for self-hosting)

## Getting started

There are little to no run-time defaults and strict validation of options
flags missed details in your config.

You can use the config generator which has opinionated options and
auto-detection for Docker details (rootful, rootless, Mac).

With Golang installed and docker-compose v2:

```shell
go run ./cmd/config-generator | tee .env
docker compose up
```

<details>
<summary> Without Golang installed </summary>

---

- Without Golang installed on Linux and docker-compose v2:

  ```shell
  docker run --rm -v `pwd`:`pwd` -w `pwd` golang:1.19.9 \
    go build ./cmd/config-generator
  ./config-generator | tee .env
  docker compose up
  ```

- Without Golang installed on an Intel based Mac and docker-compose v2:

  ```shell
  docker run --rm -v `pwd`:`pwd` -w `pwd` \
    -e GOOS=darwin -e GOARCH=amd64 golang:1.19.9 \
    go build ./cmd/config-generator
  ./config-generator | tee .env
  docker compose up
  ```

- Without Golang installed on a M1/M2 based Mac and docker-compose v2:

  ```shell
  docker run --rm -v `pwd`:`pwd` -w `pwd` \
    -e GOOS=darwin -e GOARCH=arm64 golang:1.19.9 \
    go build ./cmd/config-generator
  ./config-generator | tee .env
  docker compose up
  ```

---
</details>

> Note: For more advanced usages (e.g. email delivery), see
> `$ go run ./cmd/config-generator --help` or `$ ./config-generator --help`.

Docker-compose will build all the images, binaries and frontend assets, run
the minio setup, create the postgres schema and finally start it all up!

The first startup may take a while to complete.
Watch out for a message that says "Setup finished!".

You can now open http://localhost:8080/register in your favorite browser
and create your first account!

> Tip: Use `$ docker compose up --detach` to keep the services running in the
> background.
