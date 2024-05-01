#!/bin/bash

IMAGE_NAME="tensorflow"
DOCKER_IMAGE="tensorflow/tensorflow"
IMAGE_VERSIONS=("2.14.0-gpu" "2.15.0-gpu" "2.16.1-gpu")
IMAGE_LOWER=${IMAGE_VERSIONS[0]}
IMAGE_MIDDLE=${IMAGE_VERSIONS[1]}
IMAGE_UPPER=${IMAGE_VERSIONS[2]}
