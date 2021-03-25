#!/bin/bash

set -x

make install 


#gaiad unsafe-reset-all --home ./t6

for i in {1..1}; do
  home=./t6_$i

  rm -rf $home

  gaiad init "t6_"$i --home $home --chain-id t6
  mkdir -p $home/config/gentx
  gaiad keys add validator$i --keyring-backend test --home $home
#  read -p "Press any key to resume ..."
#  read -p "Press any key to resume ...2"
  lastacct=`gaiad keys show validator$i -a --keyring-backend test --home $home`
  valconspub=`gaiad tendermint show-validator`
  echo "lastacct:" $lastacct
  echo "valconspub:" $valconspub
  gaiad add-genesis-account $lastacct 10000000000stake --keyring-backend test --home $home
#  read -p "Press any key to resume ...3"
  gaiad gentx validator$i 10000000000stake --pubkey $valconspub --keyring-backend test --home $home --chain-id t6 --output-document $home/config/gentx/gentx-$lastacct.json
#  read -p "Press any key to resume ...4"

  gaiad collect-gentxs --home $home

  gaiad start --home $home --x-crisis-skip-assert-invariants --trace --log_level trace &

done

#  cat collect | jq -a .

#read -p "Press any key to resume ..."

set +x


#
#  gaiad tx staking create-validator \
#     --amount=1000000uatom \
#     --pubkey=$valconspub \
#     --moniker="t6" \
#     --chain-id=t6 \
#     --commission-rate="0.10" \
#     --commission-max-rate="0.20" \
#     --commission-max-change-rate="0.01" \
#     --min-self-delegation="1" \
#     --gas="auto" \
#     --gas-prices="0.025uatom" \
#     --gas-adjustment="1.80" \
#     --from=validator$i \
#     --home ./t6 \
#     --keyring-backend test
