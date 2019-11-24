#!/usr/bin/env bash
VERSION=$(git describe HEAD)
echo Setting version ${VERSION}
kubectl set image deployment/swosh swosh=edholm/swosh:${VERSION} -n swosh
kubectl get deployments swosh -n swosh -o wide
