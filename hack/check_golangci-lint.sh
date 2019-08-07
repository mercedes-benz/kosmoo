#!/bin/bash
# SPDX-License-Identifier: MIT

set -e

GOLANGCILINT_VERSION="1.17.1"
GOLANGCILINT_FILENAME="golangci-lint-${GOLANGCILINT_VERSION}-linux-amd64.tar.gz"
GOLANGCILINT_URL="https://github.com/golangci/golangci-lint/releases/download/v${GOLANGCILINT_VERSION}/${GOLANGCILINT_FILENAME}"

TMP_BIN="$(pwd)/tmp/bin"

if ! [ -x "$(command -v golangci-lint)" ]; then
  echo '[golangci-lint/prepare]: golangci-lint is not installed. Downloading to tmp/bin' >&2

  wget -q "${GOLANGCILINT_URL}"
  tar -xf "${GOLANGCILINT_FILENAME}"
  
  mkdir -p "${TMP_BIN}"
  mv "${GOLANGCILINT_FILENAME%.tar.gz}/golangci-lint" ${TMP_BIN}/
  
  export PATH="${PATH}:${TMP_BIN}"

  rm -rf "${GOLANGCILINT_FILENAME%.tar.gz}" "${GOLANGCILINT_FILENAME}"
fi

echo '[golangci-lint/prepare]: running golangci-lint' >&2
golangci-lint run --skip-dirs ./vendor
