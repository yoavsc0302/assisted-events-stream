#!/bin/bash

set -exu

TAG=$(git rev-parse --short=7 HEAD)
export IMAGE_NAME="quay.io/app-sre/assisted-events-stream"
export IMAGE_TAG=$TAG
export DOCKER_BUILDKIT=1
export REDIS_EXPORTER_IMAGE_NAME="quay.io/app-sre/assisted-redis"
export REDIS_IMAGE_NAME="quay.io/app-sre/assisted-redis-exporter"
export REDIS_IMAGE_TAG="6.2.7-debian-10-r23"
export REDIS_EXPORTER_IMAGE_TAG="1.37.0-debian-10-r63"

make docker-build
make redis-docker-build

DOCKER_CONF="$PWD/assisted/.docker"
mkdir -p "$DOCKER_CONF"
docker --config="$DOCKER_CONF" login -u="${QUAY_USER}" -p="${QUAY_TOKEN}" quay.io
docker tag "${IMAGE_NAME}:${IMAGE_TAG}" "${IMAGE_NAME}:latest"
docker --config="$DOCKER_CONF" push "${IMAGE_NAME}:${IMAGE_TAG}"
docker --config="$DOCKER_CONF" push "${IMAGE_NAME}:latest"

docker --config="$DOCKER_CONF" push "${REDIS_EXPORTER_IMAGE_NAME}:${REDIS_EXPORTER_IMAGE_TAG}"
docker --config="$DOCKER_CONF" push "${REDIS_IMAGE_NAME}:${REDIS_IMAGE_TAG}"
