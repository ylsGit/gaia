package rpc

import (
	"strings"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ethsecp256k1"
	"github.com/cosmos/cosmos-sdk/server"
	evmtypes "github.com/cosmos/cosmos-sdk/x/evm/types"
	"github.com/cosmos/gaia/v4/ethereum/rpc/backend"
	"github.com/cosmos/gaia/v4/ethereum/rpc/namespaces/eth"
	"github.com/cosmos/gaia/v4/ethereum/rpc/namespaces/eth/filters"
	"github.com/cosmos/gaia/v4/ethereum/rpc/namespaces/net"
	"github.com/cosmos/gaia/v4/ethereum/rpc/namespaces/personal"
	"github.com/cosmos/gaia/v4/ethereum/rpc/namespaces/web3"
	rpctypes "github.com/cosmos/gaia/v4/ethereum/rpc/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/spf13/viper"
	"github.com/tendermint/tendermint/libs/log"
	"golang.org/x/time/rate"
)

// RPC namespaces and API version
const (
	Web3Namespace     = "web3"
	EthNamespace      = "eth"
	PersonalNamespace = "personal"
	NetNamespace      = "net"

	apiVersion = "1.0"
)

// GetRPCAPIs returns the list of all APIs from the Ethereum namespaces
func GetRPCAPIs(clientCtx client.Context, log log.Logger, keys ...ethsecp256k1.PrivKey) []rpc.API {
	nonceLock := new(rpctypes.AddrLocker)
	rateLimiters := getRateLimiter()
	ethBackend := backend.New(clientCtx, log, rateLimiters)
	ethAPI := eth.NewAPI(clientCtx, log, ethBackend, nonceLock, keys...)
	if evmtypes.GetEnableBloomFilter() {
		server.TrapSignal(func() {
			if ethBackend != nil {
				ethBackend.Close()
			}
		})
		ethBackend.StartBloomHandlers(evmtypes.BloomBitsBlocks, evmtypes.GetIndexer().GetDB())
	}

	apis := []rpc.API{
		{
			Namespace: Web3Namespace,
			Version:   apiVersion,
			Service:   web3.NewAPI(),
			Public:    true,
		},
		{
			Namespace: EthNamespace,
			Version:   apiVersion,
			Service:   ethAPI,
			Public:    true,
		},
		{
			Namespace: EthNamespace,
			Version:   apiVersion,
			Service:   filters.NewAPI(clientCtx, ethBackend),
			Public:    true,
		},
		{
			Namespace: NetNamespace,
			Version:   apiVersion,
			Service:   net.NewAPI(clientCtx),
			Public:    true,
		},
	}

	if viper.GetBool(FlagPersonalAPI) {
		apis = append(apis, rpc.API{
			Namespace: PersonalNamespace,
			Version:   apiVersion,
			Service:   personal.NewAPI(ethAPI, log),
			Public:    false,
		})
	}
	return apis
}

func getRateLimiter() map[string]*rate.Limiter {
	rateLimitApi := viper.GetString(FlagRateLimitApi)
	rateLimitCount := viper.GetInt(FlagRateLimitCount)
	rateLimitBurst := viper.GetInt(FlagRateLimitBurst)
	if rateLimitApi == "" || rateLimitCount == 0 {
		return nil
	}
	rateLimiters := make(map[string]*rate.Limiter)
	apis := strings.Split(rateLimitApi, ",")
	for _, api := range apis {
		rateLimiters[api] = rate.NewLimiter(rate.Limit(rateLimitCount), rateLimitBurst)
	}
	return rateLimiters
}
