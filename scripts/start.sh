#!/bin/bash

KEY="mykey"
CHAINID="gaia-100"
MONIKER="localtestnet"

# stop and remove existing daemon and client data and process(es)
rm -rf ~/.gaia*
rm ./build/gaiad
rm ./gaiad.log
pkill -f "gaia*"

make build

# if $KEY exists it should be override
#"$PWD"/build/gaiad keys add $KEY --keyring-backend test --algo "eth_secp256k1"
"$PWD"/build/gaiad keys unsafe-import-eth-key "$KEY" EA59874325160B1970A3251F4BBADECEE02827AA2DAE79A5A5756CF76726784D --keyring-backend test

# Set moniker and chain-id for Ethermint (Moniker can be anything, chain-id must be an integer)
"$PWD"/build/gaiad init $MONIKER --chain-id $CHAINID

# Allocate genesis accounts (cosmos formatted addresses)
"$PWD"/build/gaiad add-genesis-account "$("$PWD"/build/gaiad keys show "$KEY" -a --keyring-backend test)" 1001000stake --keyring-backend test

# Sign genesis transaction
"$PWD"/build/gaiad gentx $KEY 1000000stake --keyring-backend test --chain-id $CHAINID

# Collect genesis tx
"$PWD"/build/gaiad collect-gentxs

# Run this to ensure everything worked and that the genesis file is setup correctly
"$PWD"/build/gaiad validate-genesis

# Start the node (remove the --pruning=nothing flag if historical queries are not needed) in background and log to file
#"$PWD"/build/gaiad start --pruning=nothing --rpc.unsafe --evm-rpc.address="0.0.0.0:8545" --keyring-backend test --log_level debug > gaiad.log 2>&1 &
"$PWD"/build/gaiad start --pruning=nothing --rpc.unsafe --evm-rpc.address="0.0.0.0:8545" --keyring-backend test --log_level debug
# Give gaiad node enough time to launch
#sleep 5
