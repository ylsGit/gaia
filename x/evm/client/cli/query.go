package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/version"

	// "strings"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	// "github.com/cosmos/cosmos-sdk/client/flags"
	// sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/cosmos/gaia/v4/x/evm/types"
)

// GetQueryCmd returns the cli query commands for this module
func GetQueryCmd() *cobra.Command {
	// Group evm queries under a subcommand
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("Querying commands for the %s module", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		GetCmdQueryEvmTxCmd(),
		GetCmdGetStorageAt(),
		GetCmdGetCode(),
		GetCmdQueryParams(),
	)

	return cmd
}

// GetCmdQueryEvmTxCmd implements the command for the query of transactions including evm
func GetCmdQueryEvmTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tx [hash]",
		Short: "Query for all transactions including evm by hash in a committed block",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
			//clientCtx, err := client.GetClientQueryContext(cmd)
			//if err != nil {
			//	return err
			//}
			//queryClient := types.NewQueryClient(clientCtx)
			//
			//
			//res, err := queryClient.(context.Background(), params)
			//if err != nil {
			//	return err
			//}
			//
			//res, err := rest.QueryTx(cliCtx, args[0])
			//if err != nil {
			//	return err
			//}
			//
			//output, ok := res.(sdk.TxResponse)
			//if !ok {
			//	// evm tx result
			//	fmt.Println(string(res.([]byte)))
			//	return nil
			//}
			//
			//if output.Empty() {
			//	return fmt.Errorf("no transaction found with hash %s", args[0])
			//}
			//
			//return cliCtx.PrintProto(output)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

// GetCmdGetStorageAt queries a key in an accounts storage
func GetCmdGetStorageAt() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "storage [account] [key]",
		Short: "Gets storage for an account at a given key",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			account, err := accountToHex(args[0])
			if err != nil {
				return err
			}

			key := formatKeyToHash(args[1])

			params := &types.QueryStorageRequest{
				Address: account,
				Key:     key,
			}

			res, err := queryClient.Storage(context.Background(), params)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

// GetCmdGetCode queries the code field of a given address
func GetCmdGetCode() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "code [account]",
		Short: "Gets code from an account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			account, err := accountToHex(args[0])
			if err != nil {
				return err
			}

			params := &types.QueryCodeRequest{
				Address: account,
			}

			res, err := queryClient.Code(context.Background(), params)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

// GetCmdQueryParams implements the params query command.
func GetCmdQueryParams() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Args:  cobra.NoArgs,
		Short: "Query the current evm parameters",
		Long: strings.TrimSpace(
			fmt.Sprintf(`Query values set as evm parameters.

Example:
$ %s query evm params
`,
				version.AppName,
			),
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.Params(context.Background(), &types.QueryParamsRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(&res.Params)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}
