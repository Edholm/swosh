#!/usr/bin/env bash
# Note that this script requires you to have a `docker context` setup called io1 for prod and io2 for staging
# E.g. docker context create io1 --docker "host=ssh://myuser@example.org"
set -e

if [ $# -eq 0 ]; then
  echo "Usage: deploy.sh <prod|staging>"
   exit 1
fi

version=$(git describe HEAD)
export TAG=${version}

if [ "$1" == "prod" ]; then
  echo "Deploying to prod using context io1"
  export SWOSH_HOSTNAME=swosh.me
  export SWOSH_SCHEMA=https
  docker-compose --context io1 --project-name swosh_prod up -d
else
  echo "Deploying to staging using context io2"
  export SWOSH_HOSTNAME=staging.swosh.me
  export SWOSH_SCHEMA=https
  docker-compose --context io2 --project-name swosh_staging up -d
fi
