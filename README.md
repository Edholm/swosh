# Swosh.me ![](https://github.com/Edholm/swosh/workflows/Build,%20test,%20and%20deploy/badge.svg)

Used for creating short links that you can send to your friends, thus allowing them to easily launch Swish with your preset information.

This is useful since https:// links are almost universally clickable.

## Requirements
* Gradle
* Java 15
* _Kotlin_ 
* _spring-boot_

## Go port (in progress)

The service is being rewritten in Go with PostgreSQL. Until it reaches parity, the Kotlin
app above is still what gets deployed. So far the Go side has the database layer, a
`swosh serve` command that applies migrations and serves `/health`, and `mongo-migrate`,
which copies the old MongoDB data into Postgres.

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
| `SWOSH_ACTUATOR_PORT` | `8081` |
| `LOG_LEVEL` | `info` (`debug`, `info`, `warn` or `error`; the old `TRACE` is not accepted) |

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
