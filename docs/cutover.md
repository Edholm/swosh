# Cutover runbook: Kotlin + MongoDB → Go + PostgreSQL

How to move production from the Kotlin service to the Go one. Kotlin and MongoDB
stay untouched until the move is signed off, so rolling back is a matter of
starting the Kotlin container again.

The site is down from step 2 (Kotlin stops) until step 6 (Go starts), usually a
few minutes. Rehearse on staging first with the same steps.

All commands run from a checkout of this repository on master, against the
remote Docker host through its context. Compose runs on the machine you deploy
from, so that machine needs Docker Compose 2.24+; the host's own compose
version doesn't matter.

## What changes for users

- Links, including their IDs, and admin users are copied. Links that have
  already expired are skipped.
- Logins are not copied. Everyone has to log in to `/admin` again, with the same
  username and password as before.
- Logins are signed cookies, valid for 12 hours. They only survive restarts when
  `SWOSH_SESSION_KEY` is set, so production must set it.
- On startup, swosh creates `SWOSH_ADMIN_USERNAME` (default `admin`) with
  `SWOSH_ADMIN_PWD` unless that user exists, so production must set a real
  password. If an `admin` user is copied from MongoDB, it keeps its old
  password.

## 0. Prepare (no downtime)

1. Merge `refactor/cutover`. The `Go` workflow pushes
   `ghcr.io/edholm/swosh:<short sha>` and `latest` from master. Without CI,
   push it with `make image-push`.

   Log in to GHCR on the machine you deploy from, with a personal access token
   that has `write:packages` (or `read:packages` if CI does the pushing).
   Compose pulls through the context with these local credentials, so the
   hosts need no login of their own. If the package is public, pulling needs
   no login at all.

   ```sh
   docker login ghcr.io -u Edholm
   ```

2. Fill in `.env.prod` (`.env.staging` for staging) from `.env.example`.
   Production needs `POSTGRES_PASSWORD`, a `SWOSH_SESSION_KEY` of at least
   32 characters and a `SWOSH_ADMIN_PWD` other than `admin`. On staging, also
   set `SWOSH_HOSTNAME=staging.swosh.me`.

   ```sh
   cp .env.example .env.prod
   openssl rand -hex 24   # POSTGRES_PASSWORD
   openssl rand -hex 32   # SWOSH_SESSION_KEY
   ```

3. Set up the shell helpers used below. On staging use `io2`, `swosh_staging`
   and `.env.staging`.

   ```sh
   ctx=io1 project=swosh_prod env=.env.prod
   dc() { docker --context $ctx compose --env-file $env --project-name $project "$@"; }
   container() {
     docker --context $ctx ps -aq \
       --filter label=com.docker.compose.project=$project \
       --filter label=com.docker.compose.service=$1
   }
   ```

4. Write down the Kotlin image that is running, for the rollback. Don't prune
   images on the host until the move is signed off.

   ```sh
   docker --context $ctx inspect -f '{{.Config.Image}}' "$(container swosh)"   # e.g. edholm/swosh:1.2.20, on Docker Hub
   ```

5. Build `mongo-migrate` and load it onto the host. It was removed from master
   after the move, so build it from the commit before
   `refactor!: remove mongo-migrate`:

   ```sh
   make tools   # installs ./bin/ko
   ko=$PWD/bin/ko
   git worktree add ../swosh-mongo-migrate \
     "$(git log -1 --format=%H --grep='^refactor!: remove mongo-migrate')~1"
   arch=$(docker --context $ctx version -f '{{.Server.Arch}}')
   (cd ../swosh-mongo-migrate &&
     KO_DOCKER_REPO=ko.local/mongo-migrate $ko build --bare --platform=linux/$arch ./cmd/mongo-migrate)
   docker save ko.local/mongo-migrate:latest | docker --context $ctx load
   ```

## 1. Back up MongoDB

Into `tmp/`, which is git-ignored.

```sh
docker --context $ctx exec "$(container mongodb)" \
  mongodump --archive --gzip --db swosh > tmp/swosh-mongo-$(date +%F).archive.gz
```

## 2. Stop Kotlin

This stops all writes. Downtime starts here.

```sh
dc stop swosh
```

## 3. Start PostgreSQL

Only Postgres; swosh must not start before the import, or it would provision
the admin user before the old one is copied.

```sh
dc up --wait postgres   # returns once it is healthy
```

## 4. Copy the data

`mongo-migrate` runs on the compose network, where MongoDB is `mongodb` and
Postgres is `postgres`. It applies the schema migrations and imports everything
in one transaction. Run it with `-dry-run` first: that rolls the transaction
back, but prints the exact counts.

```sh
migrate() {
  SWOSH_DATABASE_URL="postgres://swosh:$(grep '^POSTGRES_PASSWORD=' $env | cut -d= -f2-)@postgres:5432/swosh?sslmode=disable" \
    docker --context $ctx run --rm --network ${project}_default \
      -e SWOSH_MONGODB_URI=mongodb://mongodb/swosh -e SWOSH_DATABASE_URL \
      ko.local/mongo-migrate "$@"
}
migrate -dry-run
migrate
```

Both print a line like:

```text
msg="import finished" dry_run=false swosh_inserted=1234 swosh_existing=0 swosh_expired=5 users_inserted=2 users_existing=0
```

Running it again is safe: rows that already exist are left alone.

## 5. Check the counts

```sh
docker --context $ctx exec "$(container mongodb)" mongo --quiet swosh --eval '
  print("links", db.swosh.countDocuments({}),
        "unexpired", db.swosh.countDocuments({$or: [{expiresOn: null}, {expiresOn: {$gt: new Date()}}]}),
        "users", db.user.countDocuments({}));
  print("a link to test", db.swosh.findOne({expiresOn: null})._id)'
dc exec -T postgres psql -U swosh -d swosh -c \
  'select (select count(*) from swosh) as links, (select count(*) from users) as users'
```

Postgres should have as many links as MongoDB has unexpired ones
(`swosh_inserted`), and the same number of users. Stop and roll back if they
differ by more than a link or two that expired in between.

## 6. Start Go

Start the image CI pushed for the merged commit:

```sh
SWOSH_IMAGE=ghcr.io/edholm/swosh TAG=<short sha> dc up --wait
```

Compose replaces the Kotlin `swosh` container with the Go one and warns about
the orphan `mongodb` container. That is expected: don't pass `--remove-orphans`,
MongoDB stays for the rollback. Downtime ends here.

## 7. Smoke test

```sh
dc ps                                            # swosh and postgres "Up (healthy)"
dc logs swosh                                    # "listening" on 8080 and 8081, no errors
id=<the link from step 5>
curl -s -o /dev/null -w '%{http_code}\n' https://swosh.me/                              # 200
curl -s -o /dev/null -w '%{http_code}\n' https://swosh.me/$id                           # 200
curl -s -o /dev/null -w '%{http_code} %{redirect_url}\n' https://swosh.me/$id/redir     # 307 swish://payment/?data=…
curl -s -o /dev/null -w '%{http_code} %{content_type}\n' https://swosh.me/$id/qr        # 200 image/png
```

Then in a browser:

1. Open `https://swosh.me/<id>`: the preview shows the payee, amount and QR code.
2. Create a link on `https://swosh.me/` with a short expiry, e.g. 10 minutes, and
   open it.
3. Log in at `https://swosh.me/login` with an existing admin account. `/admin`
   should list as many links as Postgres had in step 5, plus the new one.
4. Check that Prometheus still scrapes `:8081/prometheus`.

## Rollback

Possible until MongoDB is removed in the next section. Links created or changed
while Go was running exist only in Postgres and are not carried back.

```sh
dc stop swosh
git show d700851:docker-compose.yml > tmp/kotlin-compose.yml   # the Kotlin compose file
TAG=<tag from step 0.4> SWOSH_HOSTNAME=swosh.me SWOSH_SCHEME=https SWOSH_PROVISION_USERS=false \
  docker --context $ctx compose --project-name $project -f tmp/kotlin-compose.yml up -d swosh
```

Postgres keeps running and does no harm. To try the move again, start over at
step 1. For an exact copy, remove the earlier import first:

```sh
dc rm -sf postgres
docker --context $ctx volume rm ${project}_postgres-data
```

## After sign-off

Once Go has run without problems for a while, remove MongoDB and the leftovers.
Keep the backup from step 1.

```sh
docker --context $ctx rm -f "$(container mongodb)"
docker --context $ctx volume rm ${project}_mongo-data ${project}_mongo-config
docker --context $ctx rmi ko.local/mongo-migrate <Kotlin image from step 0.4>
git worktree remove ../swosh-mongo-migrate
```
