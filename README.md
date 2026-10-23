# Swosh.me ![](https://github.com/Edholm/swosh/workflows/Go/badge.svg)

Used for creating short links that you can send to your friends, thus allowing them to easily launch Swish with your preset information.

This is useful since https:// links are almost universally clickable.

## Requirements
* Gradle
* Java 15
* _Kotlin_ 
* _spring-boot_

## Go port (in progress)

The service is being rewritten in Go with PostgreSQL. Until it reaches parity, the Kotlin
app above is still what gets deployed. So far the Go side has the database layer,
`mongo-migrate`, which copies the old MongoDB data into Postgres, and `swosh serve`, which
applies migrations and serves the public site: `POST /api/create`, the start page, the
`/{id}` preview, `/{id}/redir` and `/{id}/qr`. Expired links are deleted in the
background. `/health` and `/prometheus` are on the actuator port.

The admin pages are ported too: log in at `/login` (or with HTTP Basic) and manage links
at `/admin`. They need `ROLE_ADMIN`; editing needs `UPDATE` and deleting `DELETE`.
Existing users from MongoDB keep their passwords, but have to log in again: sessions are
signed cookies now, valid for 12 hours.

Requirements: Go 1.27 and Docker.

```sh
make tools              # install pinned sqlc, golangci-lint and migrate into ./bin
make test-integration   # start Postgres from docker-compose.dev.yml and run all tests
make run                # run `swosh serve` against that Postgres
make                    # list all targets
```

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

## Deploy
You can "deploy" Swosh using `docker-compose`. See `deploy.sh` for an example of how it can be done.

## License

See the [LICENSE](LICENSE.md) file for license rights and limitations (MIT).
