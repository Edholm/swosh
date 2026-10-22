# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Swosh creates short links that open Swish (Swedish mobile payments) with a prefilled payee, amount and message.

It is being ported from Kotlin/Spring Boot + MongoDB to Go + PostgreSQL, in steps:
1. Foundation: done.
2. `feat/public-api`: create API, preview/redirect/QR pages, expiry job, metrics.
3. `feat/admin`: login and admin CRUD.
4. `refactor/cutover`: Docker/compose/deploy, data move, removal of Kotlin and `cmd/mongo-migrate`.

Until the cutover, the Kotlin app (`src/`, Gradle, `.github/workflows/ci.yml`/`prod.yml`, `deploy.sh`, `docker-compose.yml`) is still what deploys production. Leave it alone.

Use `src/main/kotlin/pub/edholm` as the behavioural reference when porting. `SwoshHandlerTest.kt` holds test cases to carry over.

## Commands

```sh
make tools              # once: pinned sqlc, golangci-lint v2, migrate into ./bin
make generate           # after changing internal/db/migrations or internal/db/queries
make lint / make fmt
make test               # DB tests skip without SWOSH_TEST_DATABASE_URL
make test-integration   # starts Postgres (docker-compose.dev.yml) and runs everything
make check              # generate-check + lint + test, like CI
make build              # bin/swosh, bin/mongo-migrate
make run                # swosh serve against the dev Postgres
make migrate-new name=add_foo
```

Run a single DB test (after `make db-up`):

```sh
SWOSH_TEST_DATABASE_URL='postgres://swosh:swosh@localhost:5432/swosh?sslmode=disable' go test ./internal/db -run TestUpdateSwoshRenames
```

Use the tools in `./bin`, not global ones. A globally installed golangci-lint v1 can't read the v2 `.golangci.yml`.

## Architecture

- **CLI:** `cmd/swosh` only calls `internal/cli`, which is built on cobra. Root's `PersistentPreRunE` loads `internal/config` (go-envconfig) and a slog JSON logger into a shared `app` struct. Subcommands are `newXCmd(a *app)` functions registered in `NewRootCmd`. `version` overrides the hook so it works even when the environment is broken.
- **Database:** `internal/db` embeds `migrations/*.sql` (golang-migrate format). `db.Migrate` applies them on every `swosh serve` startup.
  - sqlc generates `internal/db/sqlc` from the migrations plus `queries/`. Never edit the generated code. CI runs `sqlc diff` and fails if it is stale.
- **DB tests:** `dbtest.URL(t)` gives each test a throwaway schema through `search_path`. Combine it with `db.Migrate` + `db.Connect`.
- **Data move:** `cmd/mongo-migrate` is the one-off Mongo→Postgres copy. It is a separate binary so `swosh` never links the Mongo driver.

## Data model gotchas

- The tables mirror the Spring Data documents.
  - Link IDs are usually 6 random base-32 characters; admins renamed a few to shorter IDs.
  - Almost all real links never expire (`expires_on` NULL).
- Passwords keep Spring's `{bcrypt}$2a$…` format. Strip the `{bcrypt}` prefix before checking them with `x/crypto/bcrypt`.
- There are no CHECK constraints on `amount`/`description`, on purpose: legacy rows break the API rules. Validate in Go.
- `ListSwoshes` sorts `expires_on DESC NULLS LAST` to match the old Mongo ordering.

## Conventions

- Commit messages use Conventional Commits (`feat(db): …`, `chore: …`). Branch names follow `feat/xyz`, `refactor/xyz`.
- Go 1.27 (`go.mod`). If the local Go is older, the toolchain auto-downloads.
- Environment variable names reuse the Kotlin ones where the setting still exists (`SWOSH_*`, `LOG_LEVEL`).
- `tmp/` is git-ignored and may contain real production dumps. Never commit them or print their contents.
