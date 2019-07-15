#!/usr/bin/env bash
set -e

version=$(./gradlew properties --no-daemon --console=plain -q | grep "^version:" | awk '{printf $2}')
repo="edholm/swosh"
base_tag="${repo}:${version}"
latest_tag="${repo}:latest"

function dockerbuild {
    platform="${1}"
    tag="${base_tag}-${platform}"

    echo -----------------------------------------
    echo Starting build of $tag
    docker build --pull=true --platform "${platform}" --squash --tag="${tag}" -f Dockerfile build/libs
    docker push "${tag}"
}

dockerbuild arm
dockerbuild amd64

docker manifest create --amend ${base_tag} \
    ${base_tag}-arm \
    ${base_tag}-amd64
docker manifest create --amend ${latest_tag} \
    ${base_tag}-arm \
    ${base_tag}-amd64

docker manifest push ${base_tag}
docker manifest push ${latest_tag}

echo "${base_tag} is done!"
