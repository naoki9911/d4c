#!/bin/bash

IMAGE_NAME="mysql"
DOCKER_IMAGE="mysql"
IMAGE_VERSIONS=(8.0.29 8.0.30 8.0.31)
IMAGE_LOWER=${IMAGE_VERSIONS[0]}
IMAGE_MIDDLE=${IMAGE_VERSIONS[1]}
IMAGE_UPPER=${IMAGE_VERSIONS[2]}
