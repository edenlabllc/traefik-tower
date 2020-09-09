#!/usr/bin/env bash

set -o errexit

cd /app
go mod download

golangci-lint run -v --config ./.golangci.yml