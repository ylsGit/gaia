#!/usr/bin/env bash

set -euo pipefail

go get -u github.com/tendermint/tendermint
make localnet-start
docker-compose logs -f
