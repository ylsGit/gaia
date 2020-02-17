#!/usr/bin/env bash

set -x
set -euo pipefail

read -r HEIGHT HASH <<<$(curl -sSf 'localhost:26657/commit?height=1' | jq -r '"\(.result.signed_header.header.height) \(.result.signed_header.commit.block_id.hash)"')

#docker stop gaiadnode3
#rm -rf build/node3/gaiad/data/*
#echo '{"height":"0","round":"0","step":0}' >build/node3/gaiad/data/priv_validator_state.json
gsed -ire 's/^verify_height = .*/verify_height = '"$HEIGHT"'/g' build/node3/gaiad/config/config.toml
gsed -ire 's/^verify_hash = .*/verify_hash = "'"$HASH"'"/g' build/node3/gaiad/config/config.toml
docker-compose up -d gaiadnode3
docker logs -f gaiadnode3
