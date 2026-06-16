#!/usr/bin/env bash
# Build and push the Go routing engine to ECR for linux/arm64 (Graviton).
# Requires: docker buildx, AWS CLI creds with ECR push permission.
#   AWS_REGION=us-east-1 TAG=v1 REPO=vertex-go-routing-engine ./build-push.sh
set -euo pipefail

AWS_REGION="${AWS_REGION:-us-east-1}"
TAG="${TAG:-v1}"
REPO="${REPO:-vertex-go-routing-engine}"

ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
REGISTRY="${ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"
IMAGE="${REGISTRY}/${REPO}:${TAG}"
CONTEXT="$(cd "$(dirname "$0")" && pwd)"

echo "Logging in to ${REGISTRY} ..."
aws ecr get-login-password --region "${AWS_REGION}" \
  | docker login --username AWS --password-stdin "${REGISTRY}"

echo "Building + pushing ${IMAGE} (linux/arm64) ..."
docker buildx build --platform linux/arm64 -t "${IMAGE}" --push "${CONTEXT}"

echo "Pushed ${IMAGE}"
