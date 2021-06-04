package rpc

import (
	"bufio"
	"fmt"
	sdkcrypto "github.com/cosmos/cosmos-sdk/crypto"
	"os"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/viper"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/input"
	"github.com/cosmos/cosmos-sdk/client/lcd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	cmserver "github.com/cosmos/cosmos-sdk/server"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ethsecp256k1"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/gaia/v4/ethereum/rpc/websockets"
)

const (
	flagUnlockKey = "unlock-key"
	flagWebsocket = "wsport"

	FlagPersonalAPI    = "personal-api"
	FlagRateLimitApi   = "rpc.rate-limit-api"
	FlagRateLimitCount = "rpc.rate-limit-count"
	FlagRateLimitBurst = "rpc.rate-limit-burst"
)

// RegisterRoutes creates a new server and registers the `/rpc` endpoint.
// Rpc calls are enabled based on their associated module (eg. "eth").
func RegisterRoutes(rs *lcd.RestServer) {
	server := rpc.NewServer()
	accountName := viper.GetString(cmserver.FlagUlockKey)
	accountNames := strings.Split(accountName, ",")

	var privkeys []ethsecp256k1.PrivKey
	if len(accountName) > 0 {
		var err error
		inBuf := bufio.NewReader(os.Stdin)

		keyringBackend := viper.GetString(flags.FlagKeyringBackend)
		passphrase := ""
		switch keyringBackend {
		case keyring.BackendOS:
			break
		case keyring.BackendFile:
			passphrase, err = input.GetPassword(
				"Enter password to unlock key for RPC API: ",
				inBuf)
			if err != nil {
				panic(err)
			}
		}

		privkeys, err = unlockKeyFromNameAndPassphrase(accountNames, passphrase)
		if err != nil {
			panic(err)
		}
	}

	apis := GetAPIs(rs.CliCtx, rs.Logger(), privkeys...)

	// Register all the APIs exposed by the namespace services
	// TODO: handle allowlist and private APIs
	for _, api := range apis {
		if err := server.RegisterName(api.Namespace, api.Service); err != nil {
			panic(err)
		}
	}

	// Web3 RPC API route
	rs.Mux.HandleFunc("/", server.ServeHTTP).Methods("POST", "OPTIONS")

	// start websockets server
	websocketAddr := viper.GetString(flagWebsocket)
	ws := websockets.NewServer(rs.CliCtx, rs.Logger(), websocketAddr)
	ws.Start()
}

func unlockKeyFromNameAndPassphrase(accountNames []string, passphrase string) ([]ethsecp256k1.PrivKey, error) {
	kr, err := keyring.New(
		sdk.KeyringServiceName(),
		viper.GetString(flags.FlagKeyringBackend),
		viper.GetString(cmserver.FlagUlockKeyHome),
		os.Stdin,
		hd.EthSecp256k1Option(),
	)
	if err != nil {
		return []ethsecp256k1.PrivKey{}, err
	}

	// try the for loop with array []string accountNames
	// run through the bottom code inside the for loop

	keys := make([]ethsecp256k1.PrivKey, len(accountNames))
	for i, acc := range accountNames {
		// With keyring, password is not required as it is pulled from the OS prompt
		armor, err := kr.ExportPrivKeyArmor(acc, passphrase)
		if err != nil {
			return []ethsecp256k1.PrivKey{}, err
		}

		privKey, algo, err := sdkcrypto.UnarmorDecryptPrivKey(armor, passphrase)
		if err != nil {
			return []ethsecp256k1.PrivKey{}, err
		}

		if algo != ethsecp256k1.KeyType {
			return []ethsecp256k1.PrivKey{}, fmt.Errorf("invalid key algorithm, got %s, expected %s", algo, ethsecp256k1.KeyType)
		}

		// Converts key to Ethermint secp256 implementation
		ethermintPrivKey, ok := privKey.(*ethsecp256k1.PrivKey)
		if !ok {
			panic(fmt.Sprintf("invalid private key type %T at index %d", privKey, i))
		}
		keys[i] = *ethermintPrivKey
	}

	return keys, nil
}
