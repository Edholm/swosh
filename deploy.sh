#!/usr/bin/env bash
# Deploys swosh with docker compose to prod or staging.
#
#   ./deploy.sh <prod|staging> [tag]
#
# Without a tag, the current commit is built and pushed with ko (make image-push)
# and then deployed. With a tag, that already pushed image is deployed, e.g. to
# roll back.
#
# Needs a docker context for each host, io1 for prod and io2 for staging:
#   docker context create io1 --docker "host=ssh://myuser@example.org"
# the settings in .env.prod or .env.staging (see .env.example), and a login to
# GHCR, which is used both to push and, through the context, to pull:
#   docker login ghcr.io   # a token with write:packages, or read:packages to deploy tags
set -euo pipefail
cd "$(dirname "$0")"

usage() {
  echo "Usage: deploy.sh <prod|staging> [tag]" >&2
  exit 1
}

[ $# -ge 1 ] && [ $# -le 2 ] || usage
case "$1" in
  prod)
    context=io1
    project=swosh_prod
    export SWOSH_HOSTNAME=swosh.me
    ;;
  staging)
    context=io2
    project=swosh_staging
    export SWOSH_HOSTNAME=staging.swosh.me
    ;;
  *) usage ;;
esac
export SWOSH_SCHEME=https
export SWOSH_IMAGE="${KO_DOCKER_REPO:-ghcr.io/edholm/swosh}"

env_file=".env.$1"
fail() {
  echo "$env_file: $1" >&2
  exit 1
}
[ -f "$env_file" ] || fail "missing; copy .env.example and fill it in"
grep -Eq '^POSTGRES_PASSWORD=.+' "$env_file" || fail "set POSTGRES_PASSWORD"
grep -Eq '^SWOSH_SESSION_KEY=.{32,}' "$env_file" || fail "set SWOSH_SESSION_KEY to at least 32 characters"
grep -Eq '^SWOSH_ADMIN_PWD=.+' "$env_file" && ! grep -Eq '^SWOSH_ADMIN_PWD=admin$' "$env_file" ||
  fail "set SWOSH_ADMIN_PWD to something other than admin"

if [ $# -eq 2 ]; then
  export TAG="$2"
else
  make image-push KO_DOCKER_REPO="$SWOSH_IMAGE"
  TAG=$(git rev-parse --short HEAD)
  export TAG
fi

# Compose v2 if it is installed, otherwise docker-compose v1.
if docker compose version --short 2>/dev/null | grep -Eq '^v?2\.'; then
  compose=(docker --context "$context" compose)
else
  compose=(docker-compose --context "$context")
fi

echo "Deploying $SWOSH_IMAGE:$TAG to $1 using context $context"
"${compose[@]}" --env-file "$env_file" --project-name "$project" up -d
