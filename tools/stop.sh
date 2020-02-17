#!/usr/bin/env bash

set -euo pipefail

docker-compose down
rm -rf build/node* build/snapshots/
