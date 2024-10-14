#!/bin/bash

set -exu

TAG=$(git rev-parse HEAD)
SHORT_TAG=$(git rev-parse --short=7 HEAD)
export IMAGE_NAME="quay.io/app-sre/assisted-events-stream"
export IMAGE_TAG=$TAG
export DOCKER_BUILDKIT=1

make docker-build
make redis-docker-build

DOCKER_CONF="$PWD/assisted/.docker"
mkdir -p "$DOCKER_CONF"
docker --config="$DOCKER_CONF" login -u="${QUAY_USER}" -p="${QUAY_TOKEN}" quay.io
docker tag "${IMAGE_NAME}:${IMAGE_TAG}" "${IMAGE_NAME}:${SHORT_TAG}"
docker tag "${IMAGE_NAME}:${IMAGE_TAG}" "${IMAGE_NAME}:latest"
docker --config="$DOCKER_CONF" push "${IMAGE_NAME}:${IMAGE_TAG}"
docker --config="$DOCKER_CONF" push "${IMAGE_NAME}:${SHORT_TAG}"
docker --config="$DOCKER_CONF" push "${IMAGE_NAME}:latest"
