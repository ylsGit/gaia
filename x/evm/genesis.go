package evm

import (
	"fmt"

	ethcmn "github.com/ethereum/go-ethereum/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/gaia/v4/x/evm/keeper"
	"github.com/cosmos/gaia/v4/x/evm/types"
)

// InitGenesis initialize default parameters
// and the keeper's address to pubkey map
func InitGenesis(ctx sdk.Context, k keeper.Keeper, accountKeeper types.AccountKeeper, bankKeeper types.BankKeeper, data *types.GenesisState) {
	k.SetParams(ctx, data.Params)

	csdb := types.CreateEmptyCommitStateDB(k.GenerateCSDBParams(), ctx)

	for _, account := range data.Accounts {
		address := ethcmn.HexToAddress(account.Address)
		accAddress := sdk.AccAddress(address.Bytes())

		// check that the EVM balance the matches the account balance
		acc := accountKeeper.GetAccount(ctx, accAddress)
		if acc == nil {
			panic(fmt.Errorf("account not found for address %s", account.Address))
		}

		_, ok := acc.(*types.EthAccount)
		if !ok {
			panic(
				fmt.Errorf("account %s must be an %T type, got %T",
					account.Address, &types.EthAccount{}, acc,
				),
			)
		}

		evmBalance := bankKeeper.GetBalance(ctx, accAddress, data.Params.EvmDenom)
		csdb.SetNonce(address, acc.GetSequence())
		csdb.SetBalance(address, sdk.NewDecFromBigInt(evmBalance.Amount.BigInt()).BigInt())
		csdb.SetCode(address, ethcmn.Hex2Bytes(account.Code))

		for _, storage := range account.Storage {
			k.SetStateDirectly(ctx, address, ethcmn.HexToHash(storage.Key), ethcmn.HexToHash(storage.Value))
		}
	}

	// set contract deployment whitelist into store
	//csdb.SetContractDeploymentWhitelist(data.ContractDeploymentWhitelist)

	// set contract blocked list into store
	//csdb.SetContractBlockedList(data.ContractBlockedList)

	// set state objects and code to store
	_, err := csdb.Commit(false)
	if err != nil {
		panic(err)
	}

	// set storage to store
	// NOTE: don't delete empty object to prevent import-export simulation failure
	err = csdb.Finalise(false)
	if err != nil {
		panic(err)
	}

	k.SetChainConfig(ctx, data.ChainConfig)
}

// ExportGenesis writes the current store values
// to a genesis file, which can be imported again
// with InitGenesis
func ExportGenesis(ctx sdk.Context, k keeper.Keeper, accountKeeper types.AccountKeeper) (data *types.GenesisState) {
	// nolint: prealloc
	var ethGenAccounts []types.GenesisAccount
	csdb := types.CreateEmptyCommitStateDB(k.GenerateCSDBParams(), ctx)

	accountKeeper.IterateAccounts(ctx, func(account authtypes.AccountI) bool {
		ethAccount, ok := account.(*types.EthAccount)
		if !ok {
			// ignore non EthAccounts
			return false
		}

		addr := ethAccount.EthAddress()
		code, storage := []byte(nil), types.Storage(nil)
		var err error

		code = csdb.GetCode(addr)
		if storage, err = k.GetAccountStorage(ctx, addr); err != nil {
			panic(err)
		}

		genAccount := types.GenesisAccount{
			Address: addr.String(),
			Code:    ethcmn.Bytes2Hex(code),
			Storage: storage,
		}

		ethGenAccounts = append(ethGenAccounts, genAccount)
		return false
	})

	config, _ := k.GetChainConfig(ctx)
	return &types.GenesisState{
		Accounts:    ethGenAccounts,
		ChainConfig: config,
		Params:      k.GetParams(ctx),
		//ContractDeploymentWhitelist: csdb.GetContractDeploymentWhitelist(),
		//ContractBlockedList:         csdb.GetContractBlockedList(),
	}
}
