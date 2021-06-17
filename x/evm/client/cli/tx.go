package cli

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/gaia/v4/x/evm/types"
)

// NewTxCmd returns a root CLI command handler for all x/evm transaction commands.
func NewTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "EVM transactions subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		NewSendTxCmd(),
		NewGenCreateTxCmd(),
	)

	return cmd
}

// NewSendTxCmd generates an Ethermint transaction (excludes create operations)
func NewSendTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send [to_address] [amount (in aphotons)] [<data>]",
		Short: "send transaction to address (call operations included)",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			toAddr, err := cosmosAddressFromArg(args[0])
			if err != nil {
				return errors.Wrap(err, "must provide a valid Bech32 address for to_address")
			}

			// Ambiguously decode amount from any base
			amount, err := sdk.NewDecFromStr(args[1])
			if err != nil {
				return err
			}

			var data []byte
			if len(args) > 2 {
				payload := args[2]
				if !strings.HasPrefix(payload, "0x") {
					payload = "0x" + payload
				}

				data, err = hexutil.Decode(payload)
				if err != nil {
					return err
				}
			}

			from := clientCtx.GetFromAddress()
			txf := tx.NewFactoryCLI(clientCtx, cmd.Flags()).WithTxConfig(clientCtx.TxConfig).WithAccountRetriever(clientCtx.AccountRetriever)
			_, seq, err := txf.AccountRetriever().GetAccountNumberSequence(clientCtx, from)
			if err != nil {
				return errors.Wrap(err, "Could not retrieve account sequence")
			}

			// TODO: Potentially allow overriding of gas price and gas limit
			msg := types.NewMsgEthermint(seq, &toAddr, sdk.NewIntFromBigInt(amount.BigInt()), txf.Gas(),
				sdk.NewInt(types.DefaultGasPrice), data, from)
			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewGenCreateTxCmd generates an Ethermint transaction (excludes create operations)
func NewGenCreateTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create [contract bytecode] [<amount (in aphotons)>]",
		Short: "create contract through the evm using compiled bytecode",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			payload := args[0]
			if !strings.HasPrefix(payload, "0x") {
				payload = "0x" + payload
			}

			data, err := hexutil.Decode(payload)
			if err != nil {
				return err
			}

			amount := sdk.ZeroDec()
			if len(args) > 1 {
				// Ambiguously decode amount from any base
				amount, err = sdk.NewDecFromStr(args[1])
				if err != nil {
					return errors.Wrap(err, "invalid amount")
				}
			}

			from := clientCtx.GetFromAddress()
			txf := tx.NewFactoryCLI(clientCtx, cmd.Flags()).WithTxConfig(clientCtx.TxConfig).WithAccountRetriever(clientCtx.AccountRetriever)
			_, seq, err := txf.AccountRetriever().GetAccountNumberSequence(clientCtx, from)
			if err != nil {
				return errors.Wrap(err, "Could not retrieve account sequence")
			}

			// TODO: Potentially allow overriding of gas price and gas limit
			msg := types.NewMsgEthermint(seq, nil, sdk.NewIntFromBigInt(amount.BigInt()), txf.Gas(),
				sdk.NewInt(types.DefaultGasPrice), data, from)
			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			if err = tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg); err != nil {
				return err
			}

			contractAddr := ethcrypto.CreateAddress(common.BytesToAddress(from.Bytes()), seq)
			fmt.Printf(
				"Contract will be deployed to: \nHex: %s\nCosmos Address: %s\n",
				contractAddr.Hex(),
				sdk.AccAddress(contractAddr.Bytes()),
			)
			return nil
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func cosmosAddressFromArg(addr string) (sdk.AccAddress, error) {
	if strings.HasPrefix(addr, sdk.GetConfig().GetBech32AccountAddrPrefix()) {
		// Check to see if address is Cosmos bech32 formatted
		toAddr, err := sdk.AccAddressFromBech32(addr)
		if err != nil {
			return nil, errors.Wrap(err, "invalid bech32 formatted address")
		}
		return toAddr, nil
	}

	// Strip 0x prefix if exists
	addr = strings.TrimPrefix(addr, "0x")

	return sdk.AccAddressFromHex(addr)
}
