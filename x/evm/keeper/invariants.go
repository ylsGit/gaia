package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/gaia/v4/x/evm/types"
)

const (
	balanceInvariant = "balance"
	nonceInvariant   = "nonce"
)

// RegisterInvariants registers the evm module invariants
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
	ir.RegisterRoute(types.ModuleName, balanceInvariant, BalanceInvariant(k))
	ir.RegisterRoute(types.ModuleName, nonceInvariant, NonceInvariant(k))
}

// BalanceInvariant checks that all auth module's EthAccounts in the application have the same balance
// as the EVM one.
func BalanceInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var (
			msg   string
			count int
		)

		csdb := types.CreateEmptyCommitStateDB(k.GenerateCSDBParams(), ctx)
		k.accountKeeper.IterateAccounts(ctx, func(account authtypes.AccountI) bool {
			ethAccount, ok := account.(*types.EthAccount)
			if !ok {
				// ignore non EthAccounts
				return false
			}

			accountBalance := k.bankKeeper.GetBalance(ctx, ethAccount.GetAddress(), k.GetParams(ctx).EvmDenom)
			evmBalance := csdb.GetBalance(ethAccount.EthAddress())

			if evmBalance.Cmp(sdk.NewDecFromBigInt(accountBalance.Amount.BigInt()).BigInt()) != 0 {
				count++
				msg += fmt.Sprintf(
					"\tbalance mismatch for address %s: account balance %s, evm balance %s\n",
					account.GetAddress(), accountBalance.String(), evmBalance.String(),
				)
			}

			return false
		})

		broken := count != 0

		return sdk.FormatInvariant(
			types.ModuleName, balanceInvariant,
			fmt.Sprintf("account balances mismatches found %d\n%s", count, msg),
		), broken
	}
}

// NonceInvariant checks that all auth module's EthAccounts in the application have the same nonce
// sequence as the EVM.
func NonceInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var (
			msg   string
			count int
		)

		csdb := types.CreateEmptyCommitStateDB(k.GenerateCSDBParams(), ctx)
		k.accountKeeper.IterateAccounts(ctx, func(account authtypes.AccountI) bool {
			ethAccount, ok := account.(*types.EthAccount)
			if !ok {
				// ignore non EthAccounts
				return false
			}

			evmNonce := csdb.GetNonce(ethAccount.EthAddress())

			if evmNonce != ethAccount.Sequence {
				count++
				msg += fmt.Sprintf(
					"\nonce mismatch for address %s: account nonce %d, evm nonce %d\n",
					account.GetAddress(), ethAccount.Sequence, evmNonce,
				)
			}

			return false
		})

		broken := count != 0

		return sdk.FormatInvariant(
			types.ModuleName, nonceInvariant,
			fmt.Sprintf("account nonces mismatches found %d\n%s", count, msg),
		), broken
	}
}
