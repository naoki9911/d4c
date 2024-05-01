#!/bin/bash

IMAGE_NAME="pytorch"
DOCKER_IMAGE="pytorch/pytorch"
IMAGE_VERSIONS=("2.2.0-cuda12.1-cudnn8-runtime" "2.2.1-cuda12.1-cudnn8-runtime" "2.2.2-cuda12.1-cudnn8-runtime")
IMAGE_LOWER=${IMAGE_VERSIONS[0]}
IMAGE_MIDDLE=${IMAGE_VERSIONS[1]}
IMAGE_UPPER=${IMAGE_VERSIONS[2]}
