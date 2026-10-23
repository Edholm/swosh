# Swosh.me ![](https://github.com/Edholm/swosh/workflows/Go/badge.svg)

Used for creating short links that you can send to your friends, thus allowing them to easily launch Swish with your preset information.

This is useful since https:// links are almost universally clickable.

Swosh is a Go service backed by PostgreSQL. `swosh serve` applies the database migrations
and serves the public site: `POST /api/create`, the start page, the `/{id}` preview,
`/{id}/redir` and `/{id}/qr`. Expired links are deleted in the background. `/health` and
`/prometheus` are on the actuator port.

Admins log in at `/login` (or with HTTP Basic) and manage links at `/admin`. That needs
`ROLE_ADMIN`; editing needs `UPDATE` and deleting `DELETE`. Logins are signed cookies,
valid for 12 hours.

## Requirements

* Go 1.27 (an older `go` downloads it automatically)
* Docker, with compose v2 or docker-compose 1.28+
* [ko](https://ko.build) for the image; `make tools` installs the pinned version into `./bin`

## Running

```sh
make tools              # install pinned sqlc, golangci-lint, migrate and ko into ./bin
make test-integration   # start Postgres from docker-compose.dev.yml and run all tests
make run                # run `swosh serve` against that Postgres
make                    # list all targets
```

To run the image the way production does, with its own Postgres:

```sh
make image              # build ko.local/swosh into the local Docker daemon
docker compose up       # http://localhost:8080, actuator on :8081
```

That works without any settings: the admin user is `admin`/`admin`, and a restart logs
everyone out. To change that, copy `.env.example` to `.env` and fill it in. Compose
reads it automatically and it is git-ignored. `docker-compose.dev.yml` uses its own
project name, `swosh-dev`, so the two never share containers or volumes.

## Configuration

Configuration comes from the environment:

| Variable | Default |
|---|---|
| `SWOSH_DATABASE_URL` | `postgres://swosh:swosh@localhost:5432/swosh?sslmode=disable` |
| `SERVER_PORT` | `8080` |
| `SWOSH_ACTUATOR_PORT` | `8081` |
| `LOG_LEVEL` | `info` (`debug`, `info`, `warn` or `error`; the old `TRACE` is not accepted) |
| `SWOSH_SCHEME`, `SWOSH_HOSTNAME` | `https`, `swosh.me` (the public URL of created links) |
| `SWOSH_EXPIRE_RATE` | `60000` (milliseconds between deleting expired links) |
| `SWOSH_SWISH_QR_ENDPOINT` | `https://mpc.getswish.net/qrg-swish/api/v1/prefilled` |
| `SWOSH_ENVIRONMENT`, `SWOSH_SERVER_HOSTNAME`, `SWOSH_APP_NAME` | `local`, the machine's hostname, `swosh` (metric labels) |
| `SWOSH_PROVISION_USERS` | `true` (create the admin user on startup unless it exists) |
| `SWOSH_ADMIN_USERNAME`, `SWOSH_ADMIN_PWD` | `admin`, `admin` (the provisioned user; a warning is logged while the password is `admin`) |
| `SWOSH_SESSION_KEY` | random on each start (at least 32 bytes; signs login cookies, so set it to stay logged in across restarts) |

`docker-compose.yml` sets `SWOSH_DATABASE_URL` itself, from `POSTGRES_PASSWORD`, and
passes the other settings through from `.env`; see `.env.example`.

The schema is defined in `internal/db/migrations` and the queries in
`internal/db/queries`. Run `make generate` after changing either of them, and
`make migrate-new name=<name>` to add a migration.

## API
* `POST /api/create`
Accepts `application/json` and the body is as follows:
```json
	{
		"phone": "07xxxxxx",
		"amount": 100,
		"message": "example message",
		"expireAfterSeconds": null, // (optional, 0 or null is never expire)
	}
```

## Image

The image is built with ko from `.ko.yaml`; there is no Dockerfile. It contains only the
static `swosh` binary on `cgr.dev/chainguard/static` (distroless, runs as non-root), for
linux/amd64 and linux/arm64.

```sh
make image        # ko.local/swosh:<short sha> and :latest, for the local daemon's platform
make image-push   # both platforms to KO_DOCKER_REPO, by default docker.io/edholm/swosh
```

Both tag the image with the short commit SHA and `latest`. `make image-push` refuses to
run with uncommitted changes. CI pushes the image for every commit on master.

ko starts the binary without arguments and has no way to set default ones, so the
command has to be given: `docker-compose.yml` sets `command: [serve]`, and by hand it
is `docker run ko.local/swosh serve`. Without a shell or curl in the image, health
checks run `swosh healthcheck`, which requests `/health` on the actuator port.

## Deploy

```sh
./deploy.sh <prod|staging> [tag]
```

`deploy.sh` runs `docker-compose.yml` on the host behind the Docker context `io1` (prod)
or `io2` (staging), with the settings from `.env.prod` or `.env.staging`. Without a tag
it builds and pushes the current commit with `make image-push` and deploys it; with one,
it deploys that image, e.g. to roll back. It refuses env files without
`POSTGRES_PASSWORD`, a `SWOSH_SESSION_KEY` or a real `SWOSH_ADMIN_PWD`.

The one-time move from the old Kotlin service and its MongoDB is described in
[docs/cutover.md](docs/cutover.md).

## License

See the [LICENSE](LICENSE.md) file for license rights and limitations (MIT).
