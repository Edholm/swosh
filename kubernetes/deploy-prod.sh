#!/usr/bin/env bash
VERSION=$(git describe HEAD)
REPO="edholm/swosh:${VERSION}"

# Note that this requires experimental features currently
if ! docker manifest inspect "${REPO}" > /dev/null 2>&1; then
    echo "${REPO} doesn't exist in the repository. Has it been built?"
    exit 1
fi

echo Setting version "${REPO}"

kubectl set image deployment/swosh swosh=edholm/swosh:"${VERSION}" -n swosh
kubectl get deployments swosh -n swosh -o wide
