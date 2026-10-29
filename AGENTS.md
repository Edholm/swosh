# AGENTS.md

This file provides guidance to AI coding agents when working with code in this repository.

## Project

Swosh creates short links that open Swish (Swedish mobile payments) with a prefilled payee, amount and message.

It was ported from Kotlin/Spring Boot + MongoDB to Go + PostgreSQL, and production moved over on 2026-10-29. The production deploy setup lives on the server, not in this repo.

The Kotlin service, `cmd/mongo-migrate` and the cutover runbook (`docs/cutover.md`) are only in git history now. Comments that say "like the Kotlin service" refer to its behaviour, which the Go code keeps.

## Commands

```sh
make tools              # once: pinned sqlc, golangci-lint v2, migrate, ko into ./bin
make generate           # after changing internal/db/migrations or internal/db/queries
make lint / make fmt
make test               # DB tests skip without SWOSH_TEST_DATABASE_URL
make test-integration   # starts Postgres (compose.dev.yaml) and runs everything
make check              # generate-check + lint + test, like CI
make build              # bin/swosh
make run                # swosh serve against the dev Postgres
make image              # ko.local/swosh into the local Docker daemon; then `docker compose up`
make image-push         # to KO_DOCKER_REPO (ghcr.io/edholm/swosh); needs a clean tree and `docker login ghcr.io`
make migrate-new name=add_foo
```

Run a single DB test (after `make db-up`):

```sh
SWOSH_TEST_DATABASE_URL='postgres://swosh:swosh@localhost:5432/swosh?sslmode=disable' go test ./internal/db -run TestUpdateSwoshRenames
```

Use the tools in `./bin`, not global ones. A globally installed golangci-lint v1 can't read the v2 `.golangci.yml`.

## Architecture

- **CLI:** `cmd/swosh` holds the cobra commands (`cmd_*.go`, `package main`) and a thin `main.go`. Root's `PersistentPreRunE` loads `internal/config` (go-envconfig) and a slog JSON logger into a shared `app` struct. Subcommands are `newXCmd(a *app)` functions registered in `NewRootCmd`. `version` overrides the hook so it works even when the environment is broken.
- **Database:** `internal/db` embeds `migrations/*.sql` (golang-migrate format). `db.Migrate` applies them on every `swosh serve` startup.
  - sqlc generates `internal/db/sqlc` from the migrations plus `queries/`. Never edit the generated code. CI runs `sqlc diff` and fails if it is stale.
  - `internal/db/sqlc/log.go` is hand-written: `LogValue` methods that keep payees, messages and password hashes out of the log.
- **Languages:** the public pages and API error reasons come in English and Swedish. A `lang` cookie (from the footer's switch, `GET /lang?l=sv`) picks the language, else the browser's `Accept-Language`, else English.
  - Every text is in `messages` in `internal/web/i18n.go`, both languages side by side. Templates write `{{t "key"}}`. `js.*` keys reach `swosh.js` as JSON.
  - The admin pages are English only.
- **DB tests:** `dbtest.URL(t)` gives each test a throwaway schema through `search_path`. Combine it with `db.Migrate` + `db.Connect`.
- **Image:** ko builds it from `.ko.yaml` through the Makefile, which passes `VERSION` for the `-X main.version` ldflag. There is no Dockerfile. The base is static and distroless, with no shell or curl.
  - ko clears the image's arguments, so `compose.yaml` sets `command: [serve]`. Its health check runs `swosh healthcheck`.
  - CI pushes the image to GHCR on master with ko from `ko-build/setup-ko`, which logs in with `GITHUB_TOKEN`. Keep its `version` in step with `KO_VERSION` in the Makefile.
- **Compose:** `compose.yaml` is production: swosh plus Postgres, settings from an uncommitted env file (`.env.example`). `compose.dev.yaml` is the dev/test Postgres, in its own compose project `swosh-dev`.
  - Both set their project name with top-level `name:`, and use `healthcheck.start_interval` so `up --wait` returns quickly. They need Docker Compose 2.24+ and Docker Engine 25+; docker-compose v1 can't read them.

## Data model gotchas

- The tables mirror the Spring Data documents.
  - New link IDs are three words, like `brave-otter-sings` (`internal/linkid`). Older ones are usually 6 random base-32 characters, and admins renamed a few to shorter IDs.
  - Almost all real links never expire (`expires_on` NULL).
- Passwords keep Spring's `{bcrypt}$2a$…` format. Strip the `{bcrypt}` prefix before checking them with `x/crypto/bcrypt`.
- There are no CHECK constraints on `amount`/`description`, on purpose: legacy rows break the API rules. Validate in Go.

## Conventions

- Commit messages use Conventional Commits (`feat(db): …`, `chore: …`). Branch names follow `feat/xyz`, `refactor/xyz`.
- Go 1.27 (`go.mod`). If the local Go is older, the toolchain auto-downloads.
- Log through the default slog logger that root sets up, with `slog.InfoContext` and friends whenever a context is at hand, and typed attributes (`slog.String`, `slog.Any("err", err)`), never loose key-value pairs. Tests read logs with `logtest.Capture`, which swaps the global, so they can't run in parallel.
- Environment variable names reuse the Kotlin ones where the setting still exists (`SWOSH_*`, `LOG_LEVEL`).
- `tmp/` is git-ignored and may contain real production dumps. Never commit them or print their contents.
- `.env` and `.env.*` (except `.env.example`) are git-ignored and hold secrets. Never commit them or print their contents.
