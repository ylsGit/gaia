package eth

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ethsecp256k1"
	evmtypes "github.com/cosmos/cosmos-sdk/x/evm/types"
	"github.com/cosmos/gaia/v4/ethereum/rpc/backend"
	rpctypes "github.com/cosmos/gaia/v4/ethereum/rpc/types"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/spf13/viper"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/libs/log"
	tmtypes "github.com/tendermint/tendermint/types"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/evm"
)

const (
	FlagGasLimitBuffer = "gas-limit-buffer"
)

// PublicEthereumAPI is the eth_ prefixed set of APIs in the Web3 JSON-RPC spec.
type PublicEthereumAPI struct {
	ctx          context.Context
	clientCtx    client.Context
	queryClient  evmtypes.QueryClient
	chainIDEpoch *big.Int
	logger       log.Logger
	backend      backend.Backend
	keys         []ethsecp256k1.PrivKey // unlocked keys
	nonceLock    *rpctypes.AddrLocker
	keyringLock  sync.Mutex
	gasPrice     *hexutil.Big
}

// NewAPI creates an instance of the public ETH Web3 API.
func NewAPI(
	clientCtx client.Context, log log.Logger, backend backend.Backend, nonceLock *rpctypes.AddrLocker,
	keys ...ethsecp256k1.PrivKey,
) *PublicEthereumAPI {

	epoch, err := evmtypes.ParseChainID(clientCtx.ChainID)
	if err != nil {
		panic(err)
	}

	api := &PublicEthereumAPI{
		ctx:          context.Background(),
		clientCtx:    clientCtx,
		chainIDEpoch: epoch,
		logger:       log.With("module", "json-rpc", "namespace", "eth"),
		backend:      backend,
		keys:         keys,
		nonceLock:    nonceLock,
		gasPrice:     ParseGasPrice(),
		queryClient:  evmtypes.NewQueryClient(clientCtx),
	}

	if err := api.GetKeyringInfo(); err != nil {
		api.logger.Error("failed to get keybase info", "error", err)
	}

	return api
}

// GetKeyringInfo checks if the keyring is present on the client context. If not, it creates a new
// instance and sets it to the client context for later usage.
func (api *PublicEthereumAPI) GetKeyringInfo() error {
	api.keyringLock.Lock()
	defer api.keyringLock.Unlock()

	if api.clientCtx.Keyring != nil {
		return nil
	}

	kr, err := keyring.New(
		sdk.KeyringServiceName(),
		viper.GetString(flags.FlagKeyringBackend),
		viper.GetString(evm.FlagUlockKeyHome),
		api.clientCtx.Input,
		keyring.EthSecp256k1Option(),
	)
	if err != nil {
		return err
	}

	api.clientCtx.Keyring = kr
	return nil
}

// ClientCtx returns the Cosmos SDK client context.
func (api *PublicEthereumAPI) ClientCtx() client.Context {
	return api.clientCtx
}

// GetKeys returns the Cosmos SDK client context.
func (api *PublicEthereumAPI) GetKeys() []ethsecp256k1.PrivKey {
	return api.keys
}

// SetKeys sets the given key slice to the set of private keys
func (api *PublicEthereumAPI) SetKeys(keys []ethsecp256k1.PrivKey) {
	api.keys = keys
}

// ProtocolVersion returns the supported Ethereum protocol version.
func (api *PublicEthereumAPI) ProtocolVersion() hexutil.Uint {
	api.logger.Debug("eth_protocolVersion")
	return hexutil.Uint(evmtypes.ProtocolVersion)
}

// ChainId returns the chain's identifier in hex format
func (api *PublicEthereumAPI) ChainId() (hexutil.Uint, error) { // nolint
	api.logger.Debug("eth_chainId")
	return hexutil.Uint(uint(api.chainIDEpoch.Uint64())), nil
}

// Syncing returns whether or not the current node is syncing with other peers. Returns false if not, or a struct
// outlining the state of the sync if it is.
func (api *PublicEthereumAPI) Syncing() (interface{}, error) {
	api.logger.Debug("eth_syncing")

	status, err := api.clientCtx.Client.Status(api.ctx)
	if err != nil {
		return false, err
	}

	if !status.SyncInfo.CatchingUp {
		return false, nil
	}

	return map[string]interface{}{
		"startingBlock": hexutil.Uint64(status.SyncInfo.EarliestBlockHeight),
		"currentBlock":  hexutil.Uint64(status.SyncInfo.LatestBlockHeight),
		"highestBlock":  hexutil.Uint64(0), // NA
		// "pulledStates":  nil, // NA
		// "knownStates":   nil, // NA
	}, nil
}

// Coinbase is the address that staking rewards will be send to (alias for Etherbase).
func (api *PublicEthereumAPI) Coinbase() (common.Address, error) {
	api.logger.Debug("eth_coinbase")

	node, err := api.clientCtx.GetNode()
	if err != nil {
		return common.Address{}, err
	}

	status, err := node.Status(api.ctx)
	if err != nil {
		return common.Address{}, err
	}

	return common.BytesToAddress(status.ValidatorInfo.Address.Bytes()), nil
}

// Mining returns whether or not this node is currently mining. Always false.
func (api *PublicEthereumAPI) Mining() bool {
	api.logger.Debug("eth_mining")
	return false
}

// Hashrate returns the current node's hashrate. Always 0.
func (api *PublicEthereumAPI) Hashrate() hexutil.Uint64 {
	api.logger.Debug("eth_hashrate")
	return 0
}

// GasPrice returns the current gas price based on Ethermint's gas price oracle.
func (api *PublicEthereumAPI) GasPrice() *hexutil.Big {
	api.logger.Debug("eth_gasPrice")
	return api.gasPrice
}

// Accounts returns the list of accounts available to this node.
func (api *PublicEthereumAPI) Accounts() ([]common.Address, error) {
	api.logger.Debug("eth_accounts")
	api.keyringLock.Lock()
	defer api.keyringLock.Unlock()

	addresses := make([]common.Address, 0) // return [] instead of nil if empty

	infos, err := api.clientCtx.Keyring.List()
	if err != nil {
		return addresses, err
	}

	for _, info := range infos {
		addressBytes := info.GetPubKey().Address().Bytes()
		addresses = append(addresses, common.BytesToAddress(addressBytes))
	}

	return addresses, nil
}

// BlockNumber returns the current block number.
func (api *PublicEthereumAPI) BlockNumber() (hexutil.Uint64, error) {
	api.logger.Debug("eth_blockNumber")
	return api.backend.BlockNumber()
}

// GetBalance returns the provided account's balance up to the provided block number.
func (api *PublicEthereumAPI) GetBalance(address common.Address, blockNum rpctypes.BlockNumber) (*hexutil.Big, error) {
	api.logger.Debug("eth_getBalance", "address", address, "block number", blockNum)

	req := &evmtypes.QueryBalanceRequest{
		Address: address.String(),
	}

	res, err := api.queryClient.Balance(rpctypes.ContextWithHeight(blockNum.Int64()), req)
	if err != nil {
		return nil, err
	}

	val, err := evmtypes.UnmarshalBigInt(res.Balance)
	if err != nil {
		return nil, err
	}

	if blockNum != rpctypes.PendingBlockNumber {
		return (*hexutil.Big)(val), nil
	}

	// update the address balance with the pending transactions value (if applicable)
	pendingTxs, err := api.backend.UserPendingTransactions(address.String(), -1)
	if err != nil {
		return nil, err
	}

	for _, tx := range pendingTxs {
		if tx == nil {
			continue
		}

		if tx.From == address {
			val = new(big.Int).Sub(val, tx.Value.ToInt())
		}
		if *tx.To == address {
			val = new(big.Int).Add(val, tx.Value.ToInt())
		}
	}

	return (*hexutil.Big)(val), nil
}

// GetStorageAt returns the contract storage at the given address, block number, and key.
func (api *PublicEthereumAPI) GetStorageAt(address common.Address, key string, blockNum rpctypes.BlockNumber) (hexutil.Bytes, error) {
	api.logger.Debug("eth_getStorageAt", "address", address, "key", key, "block number", blockNum)

	req := &evmtypes.QueryStorageRequest{
		Address: address.String(),
		Key:     key,
	}

	res, err := api.queryClient.Storage(rpctypes.ContextWithHeight(blockNum.Int64()), req)
	if err != nil {
		return nil, err
	}

	return res.Value, nil
}

// GetTransactionCount returns the number of transactions at the given address up to the given block number.
func (api *PublicEthereumAPI) GetTransactionCount(address common.Address, blockNum rpctypes.BlockNumber) (*hexutil.Uint64, error) {
	api.logger.Debug("eth_getTransactionCount", "address", address, "block number", blockNum)

	clientCtx := api.clientCtx
	pending := blockNum == rpctypes.PendingBlockNumber

	// pass the given block height to the context if the height is not pending or latest
	if !pending && blockNum != rpctypes.LatestBlockNumber {
		clientCtx = api.clientCtx.WithHeight(blockNum.Int64())
	}

	nonce, err := api.accountNonce(clientCtx, address, pending)
	if err != nil {
		return nil, err
	}

	n := hexutil.Uint64(nonce)
	return &n, nil
}

// GetBlockTransactionCountByHash returns the number of transactions in the block identified by hash.
func (api *PublicEthereumAPI) GetBlockTransactionCountByHash(hash common.Hash) *hexutil.Uint {
	api.logger.Debug("eth_getBlockTransactionCountByHash", "hash", hash)

	resBlock, err := api.clientCtx.Client.BlockByHash(api.ctx, hash.Bytes())
	if err != nil {
		return nil
	}

	n := hexutil.Uint(len(resBlock.Block.Txs))
	return &n
}

// GetBlockTransactionCountByNumber returns the number of transactions in the block identified by its height.
func (api *PublicEthereumAPI) GetBlockTransactionCountByNumber(blockNum rpctypes.BlockNumber) *hexutil.Uint {
	api.logger.Debug("eth_getBlockTransactionCountByNumber", "block number", blockNum)

	var (
		height  int64
		err     error
		txCount hexutil.Uint
		txs     int
	)

	switch blockNum {
	case rpctypes.PendingBlockNumber:
		height, err = api.backend.LatestBlockNumber()
		if err != nil {
			return nil
		}
		resBlock, err := api.clientCtx.Client.Block(api.ctx, &height)
		if err != nil {
			return nil
		}
		// get the pending transaction count
		pendingCnt, err := api.backend.PendingTransactionCnt()
		if err != nil {
			return nil
		}
		txs = len(resBlock.Block.Txs) + pendingCnt
	case rpctypes.LatestBlockNumber:
		height, err = api.backend.LatestBlockNumber()
		if err != nil {
			return nil
		}
		resBlock, err := api.clientCtx.Client.Block(api.ctx, &height)
		if err != nil {
			return nil
		}
		txs = len(resBlock.Block.Txs)
	default:
		height = blockNum.Int64()
		resBlock, err := api.clientCtx.Client.Block(api.ctx, &height)
		if err != nil {
			return nil
		}
		txs = len(resBlock.Block.Txs)
	}

	txCount = hexutil.Uint(txs)
	return &txCount
}

// GetUncleCountByBlockHash returns the number of uncles in the block idenfied by hash. Always zero.
func (api *PublicEthereumAPI) GetUncleCountByBlockHash(_ common.Hash) hexutil.Uint {
	return 0
}

// GetUncleCountByBlockNumber returns the number of uncles in the block idenfied by number. Always zero.
func (api *PublicEthereumAPI) GetUncleCountByBlockNumber(_ rpctypes.BlockNumber) hexutil.Uint {
	return 0
}

// GetCode returns the contract code at the given address and block number.
func (api *PublicEthereumAPI) GetCode(address common.Address, blockNumber rpctypes.BlockNumber) (hexutil.Bytes, error) {
	api.logger.Debug("eth_getCode", "address", address, "block number", blockNumber)

	req := &evmtypes.QueryCodeRequest{
		Address: address.String(),
	}

	res, err := api.queryClient.Code(rpctypes.ContextWithHeight(blockNumber.Int64()), req)
	if err != nil {
		return nil, err
	}

	return res.Code, nil
}

// GetTransactionLogs returns the logs given a transaction hash.
func (api *PublicEthereumAPI) GetTransactionLogs(txHash common.Hash) ([]*ethtypes.Log, error) {
	api.logger.Debug("eth_getTransactionLogs", "hash", txHash)
	return api.backend.GetTransactionLogs(txHash)
}

// Sign signs the provided data using the private key of address via Geth's signature standard.
func (api *PublicEthereumAPI) Sign(address common.Address, data hexutil.Bytes) (hexutil.Bytes, error) {
	api.logger.Debug("eth_sign", "address", address, "data", data)
	// TODO: Change this functionality to find an unlocked account by address

	key, exist := rpctypes.GetKeyByAddress(api.keys, address)
	if !exist {
		return nil, keystore.ErrLocked
	}

	// Sign the requested hash with the wallet
	sig, err := crypto.Sign(accounts.TextHash(data), key.ToECDSA())
	if err != nil {
		return nil, err
	}

	sig[crypto.RecoveryIDOffset] += 27 // transform V from 0/1 to 27/28

	return sig, nil
}

// SendTransaction sends an Ethereum transaction.
func (api *PublicEthereumAPI) SendTransaction(args rpctypes.SendTxArgs) (common.Hash, error) {
	api.logger.Debug("eth_sendTransaction", "args", args)
	// TODO: Change this functionality to find an unlocked account by address

	key, exist := rpctypes.GetKeyByAddress(api.keys, *args.From)
	if !exist {
		api.logger.Debug("failed to find key in keyring", "key", args.From)
		return common.Hash{}, keystore.ErrLocked
	}

	// Mutex lock the address' nonce to avoid assigning it to multiple requests
	if args.Nonce == nil {
		api.nonceLock.LockAddr(*args.From)
		defer api.nonceLock.UnlockAddr(*args.From)
	}

	// Assemble transaction from fields
	tx, err := api.generateFromArgs(args)
	if err != nil {
		api.logger.Debug("failed to generate tx", "error", err)
		return common.Hash{}, err
	}

	if err := tx.ValidateBasic(); err != nil {
		api.logger.Debug("tx failed basic validation", "error", err)
		return common.Hash{}, err
	}

	// Sign transaction
	if err := tx.Sign(api.chainIDEpoch, key.ToECDSA()); err != nil {
		api.logger.Debug("failed to sign tx", "error", err)
		return common.Hash{}, err
	}

	// Encode transaction by default Tx encoder
	txEncoder := api.clientCtx.TxConfig.TxEncoder()
	txBytes, err := txEncoder(tx)
	if err != nil {
		return common.Hash{}, err
	}

	// Broadcast transaction in sync mode (default)
	// NOTE: If error is encountered on the node, the broadcast will not return an error
	res, err := api.clientCtx.BroadcastTx(txBytes)
	if err != nil {
		return common.Hash{}, err
	}

	if res.Code != abci.CodeTypeOK {
		return CheckError(res)
	}

	// Return transaction hash
	return common.HexToHash(res.TxHash), nil
}

// SendRawTransaction send a raw Ethereum transaction.
func (api *PublicEthereumAPI) SendRawTransaction(data hexutil.Bytes) (common.Hash, error) {
	api.logger.Debug("eth_sendRawTransaction", "data", data)
	tx := new(evmtypes.MsgEthereumTx)

	// RLP decode raw transaction bytes
	if err := rlp.DecodeBytes(data, tx); err != nil {
		// Return nil is for when gasLimit overflows uint64
		return common.Hash{}, err
	}

	// Encode transaction by default Tx encoder
	txEncoder := api.clientCtx.TxConfig.TxEncoder()
	txBytes, err := txEncoder(tx)
	if err != nil {
		return common.Hash{}, err
	}

	// TODO: Possibly log the contract creation address (if recipient address is nil) or tx data
	// If error is encountered on the node, the broadcast will not return an error
	res, err := api.clientCtx.BroadcastTx(txBytes)
	if err != nil {
		return common.Hash{}, err
	}

	if res.Code != abci.CodeTypeOK {
		return CheckError(res)
	}
	// Return transaction hash
	return common.HexToHash(res.TxHash), nil
}

// Call performs a raw contract call.
func (api *PublicEthereumAPI) Call(args rpctypes.CallArgs, blockNr rpctypes.BlockNumber, _ *map[common.Address]rpctypes.Account) (hexutil.Bytes, error) {
	api.logger.Debug("eth_call", "args", args, "block number", blockNr)
	simRes, err := api.doCall(args, blockNr, big.NewInt(evmtypes.DefaultRPCGasLimit), false)
	if err != nil {
		return []byte{}, TransformDataError(err, "eth_call")
	}

	data, err := evmtypes.DecodeTxResponse(simRes.Result.Data)
	if err != nil {
		return []byte{}, TransformDataError(err, "eth_call")
	}

	return (hexutil.Bytes)(data.Ret), nil
}

// DoCall performs a simulated call operation through the evmtypes. It returns the
// estimated gas used on the operation or an error if fails.
func (api *PublicEthereumAPI) doCall(
	args rpctypes.CallArgs, blockNum rpctypes.BlockNumber, globalGasCap *big.Int, isEstimate bool,
) (*sdk.SimulationResponse, error) {
	// Set sender address or use a default if none specified
	var addr common.Address

	if args.From == nil {
		addrs, err := api.Accounts()
		if err == nil && len(addrs) > 0 {
			addr = addrs[0]
		}
	} else {
		addr = *args.From
	}

	nonce := uint64(0)
	if isEstimate && args.To == nil && args.Data != nil {
		//only get real nonce when estimate gas and the action is contract deploy
		nonce, _ = api.accountNonce(api.clientCtx, addr, true)
	}

	// Set default gas & gas price if none were set
	// Change this to uint64(math.MaxUint64 / 2) if gas cap can be configured
	gas := uint64(evmtypes.DefaultRPCGasLimit)
	if args.Gas != nil {
		gas = uint64(*args.Gas)
	}
	if globalGasCap != nil && globalGasCap.Uint64() < gas {
		api.logger.Debug("Caller gas above allowance, capping", "requested", gas, "cap", globalGasCap)
		gas = globalGasCap.Uint64()
	}

	// Set gas price using default or parameter if passed in
	gasPrice := new(big.Int).SetUint64(evmtypes.DefaultGasPrice)
	if args.GasPrice != nil {
		gasPrice = args.GasPrice.ToInt()
	}

	// Set value for transaction
	value := new(big.Int)
	if args.Value != nil {
		value = args.Value.ToInt()
	}

	// Set Data if provided
	var data []byte
	if args.Data != nil {
		data = []byte(*args.Data)
	}

	tx := evmtypes.NewMsgEthereumTx(nonce, args.To, value, gas, gasPrice, data)
	tx.From = args.From.String()

	if err := tx.ValidateBasic(); err != nil {
		return nil, err
	}

	txEncoder := api.clientCtx.TxConfig.TxEncoder()
	txBytes, err := txEncoder(tx)
	if err != nil {
		return nil, err
	}

	// Transaction simulation through query
	query := abci.RequestQuery{
		Path:   "/app/simulate",
		Data:   txBytes,
		Height: blockNum.Int64(),
	}

	queryResult, err := api.clientCtx.QueryABCI(query)
	if err != nil {
		return nil, err
	}

	var simResponse sdk.SimulationResponse
	err = jsonpb.Unmarshal(strings.NewReader(string(queryResult.Value)), &simResponse)
	if err != nil {
		return nil, err
	}

	return &simResponse, nil
}

// EstimateGas returns an estimate of gas usage for the given smart contract call.
// It adds 1,000 gas to the returned value instead of using the gas adjustment
// param from the SDK.
func (api *PublicEthereumAPI) EstimateGas(args rpctypes.CallArgs) (hexutil.Uint64, error) {
	api.logger.Debug("eth_estimateGas", "args", args)
	simResponse, err := api.doCall(args, 0, big.NewInt(evmtypes.DefaultRPCGasLimit), true)
	if err != nil {
		return 0, TransformDataError(err, "eth_estimateGas")
	}

	// TODO: change 1000 buffer for more accurate buffer (eg: SDK's gasAdjusted)
	estimatedGas := simResponse.GasInfo.GasUsed
	upperPercentage := viper.GetUint64(FlagGasLimitBuffer)
	gasBuffer := estimatedGas / 100 * upperPercentage
	gas := estimatedGas + gasBuffer

	return hexutil.Uint64(gas), nil
}

// GetBlockByHash returns the block identified by hash.
func (api *PublicEthereumAPI) GetBlockByHash(hash common.Hash, fullTx bool) (interface{}, error) {
	api.logger.Debug("eth_getBlockByHash", "hash", hash, "full", fullTx)

	block, err := api.backend.GetBlockByHash(hash, fullTx)
	if err != nil {
		return nil, TransformDataError(err, RPCEthGetBlockByHash)
	}
	return block, nil
}

// GetBlockByNumber returns the block identified by number.
func (api *PublicEthereumAPI) GetBlockByNumber(blockNum rpctypes.BlockNumber, fullTx bool) (interface{}, error) {
	api.logger.Debug("eth_getBlockByNumber", "number", blockNum, "full", fullTx)

	var blockTxs interface{}
	if blockNum != rpctypes.PendingBlockNumber {
		return api.backend.GetBlockByNumber(blockNum, fullTx)
	}

	height, err := api.backend.LatestBlockNumber()
	if err != nil {
		return nil, err
	}

	// latest block info
	latestBlock, err := api.clientCtx.Client.Block(api.ctx, &height)
	if err != nil {
		return nil, err
	}

	// number of pending txs queried from the mempool
	limit := 1000
	unconfirmedTxs, err := api.clientCtx.Client.UnconfirmedTxs(api.ctx, &limit)
	if err != nil {
		return nil, err
	}

	pendingTxs, gasUsed, ethTxs, err := rpctypes.EthTransactionsFromTendermint(api.clientCtx, unconfirmedTxs.Txs, common.BytesToHash(latestBlock.Block.Hash()), uint64(height))
	if err != nil {
		return nil, err
	}

	if fullTx {
		blockTxs = ethTxs
	} else {
		blockTxs = pendingTxs
	}

	return rpctypes.FormatBlock(
		tmtypes.Header{
			Version:         latestBlock.Block.Version,
			ChainID:         api.clientCtx.ChainID,
			Height:          height + 1,
			Time:            time.Unix(0, 0),
			LastBlockID:     latestBlock.Block.LastBlockID,
			ValidatorsHash:  latestBlock.Block.NextValidatorsHash,
			ProposerAddress: latestBlock.Block.ProposerAddress,
		},
		0,
		latestBlock.Block.Hash(),
		0,
		gasUsed,
		blockTxs,
		ethtypes.Bloom{},
	), nil

}

// GetTransactionByHash returns the transaction identified by hash.
func (api *PublicEthereumAPI) GetTransactionByHash(hash common.Hash) (*rpctypes.Transaction, error) {
	api.logger.Debug("eth_getTransactionByHash", "hash", hash)

	tx, err := api.clientCtx.Client.Tx(api.ctx, hash.Bytes(), false)
	if err != nil {
		// check if the tx is on the mempool
		pendingTx, pendingErr := api.PendingTransactionsByHash(hash)
		if pendingErr != nil {
			return nil, err
		}
		return pendingTx, nil
	}

	// Can either cache or just leave this out if not necessary
	block, err := api.clientCtx.Client.Block(api.ctx, &tx.Height)
	if err != nil {
		return nil, err
	}

	blockHash := common.BytesToHash(block.Block.Hash())

	ethTx, err := rpctypes.RawTxToEthTx(api.clientCtx, tx.Tx)
	if err != nil {
		return nil, err
	}

	height := uint64(tx.Height)
	return rpctypes.NewTransaction(ethTx, common.BytesToHash(tx.Tx.Hash()), blockHash, height, uint64(tx.Index))
}

// GetTransactionByBlockHashAndIndex returns the transaction identified by hash and index.
func (api *PublicEthereumAPI) GetTransactionByBlockHashAndIndex(hash common.Hash, idx hexutil.Uint) (*rpctypes.Transaction, error) {
	api.logger.Debug("eth_getTransactionByBlockHashAndIndex", "hash", hash, "index", idx)

	resBlock, err := api.clientCtx.Client.BlockByHash(api.ctx, hash.Bytes())
	if err != nil {
		return nil, nil
	}

	return api.getTransactionByBlockAndIndex(resBlock.Block, idx)
}

// GetTransactionByBlockNumberAndIndex returns the transaction identified by number and index.
func (api *PublicEthereumAPI) GetTransactionByBlockNumberAndIndex(blockNum rpctypes.BlockNumber, idx hexutil.Uint) (*rpctypes.Transaction, error) {
	api.logger.Debug("eth_getTransactionByBlockNumberAndIndex", "number", blockNum, "index", idx)

	var (
		height int64
		err    error
	)

	switch blockNum {
	case rpctypes.PendingBlockNumber:
		// get all the EVM pending txs
		pendingTxs, err := api.backend.PendingTransactions()
		if err != nil {
			return nil, err
		}

		// return if index out of bounds
		if uint64(idx) >= uint64(len(pendingTxs)) {
			return nil, nil
		}

		// change back to pendingTxs[idx] once pending queue is fixed.
		return pendingTxs[int(idx)], nil

	case rpctypes.LatestBlockNumber:
		height, err = api.backend.LatestBlockNumber()
		if err != nil {
			return nil, err
		}

	default:
		height = blockNum.Int64()
	}

	resBlock, err := api.clientCtx.Client.Block(api.ctx, &height)
	if err != nil {
		return nil, err
	}

	return api.getTransactionByBlockAndIndex(resBlock.Block, idx)
}

func (api *PublicEthereumAPI) getTransactionByBlockAndIndex(block *tmtypes.Block, idx hexutil.Uint) (*rpctypes.Transaction, error) {
	// return if index out of bounds
	if uint64(idx) >= uint64(len(block.Txs)) {
		return nil, nil
	}

	ethTx, err := rpctypes.RawTxToEthTx(api.clientCtx, block.Txs[idx])
	if err != nil {
		// return nil error if the transaction is not a MsgEthereumTx
		return nil, nil
	}

	height := uint64(block.Height)
	txHash := common.BytesToHash(block.Txs[idx].Hash())
	blockHash := common.BytesToHash(block.Hash())
	return rpctypes.NewTransaction(ethTx, txHash, blockHash, height, uint64(idx))
}

// GetTransactionReceipt returns the transaction receipt identified by hash.
func (api *PublicEthereumAPI) GetTransactionReceipt(hash common.Hash) (interface{}, error) {
	api.logger.Debug("eth_getTransactionReceipt", "hash", hash)

	ctx := rpctypes.ContextWithHeight(int64(0))
	tx, err := api.clientCtx.Client.Tx(ctx, hash.Bytes(), false)
	if err != nil {
		// Return nil for transaction when not found
		return nil, nil
	}

	// Query block for consensus hash
	block, err := api.clientCtx.Client.Block(api.ctx, &tx.Height)
	if err != nil {
		return nil, err
	}

	blockHash := common.BytesToHash(block.Block.Hash())

	// Convert tx bytes to eth transaction
	ethTx, err := rpctypes.RawTxToEthTx(api.clientCtx, tx.Tx)
	if err != nil {
		return nil, err
	}

	from, err := ethTx.VerifySig(ethTx.ChainID())
	if err != nil {
		return nil, err
	}

	cumulativeGasUsed := uint64(tx.TxResult.GasUsed)
	if tx.Index != 0 {
		cumulativeGasUsed += rpctypes.GetBlockCumulativeGas(api.clientCtx, block.Block, int(tx.Index))
	}

	// Set status codes based on tx result
	var status hexutil.Uint
	if tx.TxResult.IsOK() {
		status = hexutil.Uint(1)
	} else {
		status = hexutil.Uint(0)
	}

	txData := tx.TxResult.GetData()

	data, err := evmtypes.DecodeTxResponse(txData)
	if err != nil {
		status = 0 // transaction failed
	}

	if len(data.Logs) == 0 {
		data.Logs = []*evmtypes.Log{}
	}
	contractAddr := &data.ContractAddress
	if data.ContractAddress == common.HexToAddress("0x00000000000000000000").String() {
		contractAddr = nil
	}

	receipt := map[string]interface{}{
		// Consensus fields: These fields are defined by the Yellow Paper
		"status":            status,
		"cumulativeGasUsed": hexutil.Uint64(cumulativeGasUsed),
		"logsBloom":         data.Bloom,
		"logs":              data.Logs,

		// Implementation fields: These fields are added by geth when processing a transaction.
		// They are stored in the chain database.
		"transactionHash": hash,
		"contractAddress": contractAddr,
		"gasUsed":         hexutil.Uint64(tx.TxResult.GasUsed),

		// Inclusion information: These fields provide information about the inclusion of the
		// transaction corresponding to this receipt.
		"blockHash":        blockHash,
		"blockNumber":      hexutil.Uint64(tx.Height),
		"transactionIndex": hexutil.Uint64(tx.Index),

		// sender and receiver (contract or EOA) addresses
		"from": from,
		"to":   ethTx.To(),
	}
	return receipt, nil
}

// PendingTransactions returns the transactions that are in the transaction pool
// and have a from address that is one of the accounts this node manages.
func (api *PublicEthereumAPI) PendingTransactions() ([]*rpctypes.Transaction, error) {
	api.logger.Debug("eth_pendingTransactions")
	return api.backend.PendingTransactions()
}

func (api *PublicEthereumAPI) PendingTransactionsByHash(target common.Hash) (*rpctypes.Transaction, error) {
	api.logger.Debug("eth_pendingTransactionsByHash")
	return api.backend.PendingTransactionsByHash(target)
}

// GetUncleByBlockHashAndIndex returns the uncle identified by hash and index. Always returns nil.
func (api *PublicEthereumAPI) GetUncleByBlockHashAndIndex(hash common.Hash, idx hexutil.Uint) map[string]interface{} {
	return nil
}

// GetUncleByBlockNumberAndIndex returns the uncle identified by number and index. Always returns nil.
func (api *PublicEthereumAPI) GetUncleByBlockNumberAndIndex(number hexutil.Uint, idx hexutil.Uint) map[string]interface{} {
	return nil
}

// GetProof returns an account object with proof and any storage proofs
func (api *PublicEthereumAPI) GetProof(address common.Address, storageKeys []string, blockNumber rpctypes.BlockNumber) (*rpctypes.AccountResult, error) {
	api.logger.Debug("eth_getProof", "address", address, "keys", storageKeys, "number", blockNumber)

	//clientCtx := api.clientCtx.WithHeight(int64(block))

	storageProofs := make([]rpctypes.StorageResult, len(storageKeys))
	for i, k := range storageKeys {
		data := append(evmtypes.AddressStoragePrefix(address), getStorageByAddressKey(address, common.HexToHash(k).Bytes()).Bytes()...)
		// Get value for key
		req := abci.RequestQuery{
			Path:   fmt.Sprintf("store/%s/key", evmtypes.StoreKey),
			Data:   data,
			Height: int64(blockNumber),
			Prove:  true,
		}

		vRes, err := api.clientCtx.QueryABCI(req)
		if err != nil {
			return nil, err
		}

		var value evmtypes.QueryStorageResponse
		value.Value = vRes.GetValue()

		// check for proof
		proof := vRes.GetProofOps()
		proofStr := new(merkle.Proof).String()
		if proof != nil {
			proofStr = proof.String()
		}

		storageProofs[i] = rpctypes.StorageResult{
			Key:   k,
			Value: (*hexutil.Big)(common.BytesToHash(value.Value).Big()),
			Proof: []string{proofStr},
		}
	}

	req := abci.RequestQuery{
		Path:   fmt.Sprintf("store/%s/key", authtypes.StoreKey),
		Data:   authtypes.AddressStoreKey(sdk.AccAddress(address.Bytes())),
		Height: int64(blockNumber),
		Prove:  true,
	}

	res, err := api.clientCtx.QueryABCI(req)
	if err != nil {
		return nil, err
	}

	// check for proof
	accountProof := res.GetProofOps()
	accProofStr := new(merkle.Proof).String()
	if accountProof != nil {
		accProofStr = accountProof.String()
	}

	// query EVM account
	accReq := &evmtypes.QueryAccountRequest{
		Address: address.String(),
	}

	accRes, err := api.queryClient.Account(rpctypes.ContextWithHeight(blockNumber.Int64()), accReq)
	if err != nil {
		return nil, err
	}

	return &rpctypes.AccountResult{
		Address:      address,
		AccountProof: []string{accProofStr},
		Balance:      (*hexutil.Big)(evmtypes.MustUnmarshalBigInt(accRes.Balance)),
		CodeHash:     common.BytesToHash(accRes.CodeHash),
		Nonce:        hexutil.Uint64(accRes.Nonce),
		StorageHash:  common.Hash{}, // Ethermint doesn't have a storage hash
		StorageProof: storageProofs,
	}, nil
}

// generateFromArgs populates tx message with args (used in RPC API)
func (api *PublicEthereumAPI) generateFromArgs(args rpctypes.SendTxArgs) (*evmtypes.MsgEthereumTx, error) {
	var (
		nonce, gasLimit uint64
		err             error
	)

	amount := (*big.Int)(args.Value)
	gasPrice := (*big.Int)(args.GasPrice)

	if args.GasPrice == nil {
		// Set default gas price
		// TODO: Change to min gas price from context once available through server/daemon
		gasPrice = ParseGasPrice().ToInt()
	}

	// get the nonce from the account retriever and the pending transactions
	nonce, err = api.accountNonce(api.clientCtx, *args.From, true)
	if err != nil {
		return nil, err
	}

	if args.Nonce != nil {
		if nonce != (uint64)(*args.Nonce) {
			return nil, fmt.Errorf(fmt.Sprintf("invalid nonce; got %d, expected %d", (uint64)(*args.Nonce), nonce))
		}
	}

	if args.Data != nil && args.Input != nil && !bytes.Equal(*args.Data, *args.Input) {
		return nil, errors.New("both 'data' and 'input' are set and not equal. Please use 'input' to pass transaction call data")
	}

	// Sets input to either Input or Data, if both are set and not equal error above returns
	var input hexutil.Bytes
	if args.Input != nil {
		input = *args.Input
	} else if args.Data != nil {
		input = *args.Data
	}

	if args.To == nil && len(input) == 0 {
		// Contract creation
		return nil, fmt.Errorf("contract creation without any data provided")
	}

	if args.Gas == nil {
		callArgs := rpctypes.CallArgs{
			From:     args.From,
			To:       args.To,
			Gas:      args.Gas,
			GasPrice: args.GasPrice,
			Value:    args.Value,
			Data:     &input,
		}
		gl, err := api.EstimateGas(callArgs)
		if err != nil {
			return nil, err
		}
		gasLimit = uint64(gl)
	} else {
		gasLimit = (uint64)(*args.Gas)
	}
	msg := evmtypes.NewMsgEthereumTx(nonce, args.To, amount, gasLimit, gasPrice, input)
	msg.From = args.From.String()

	return msg, nil
}

// pendingMsgs constructs an array of sdk.Msg. This method will check pending transactions and convert
// those transactions into ethermint messages.
func (api *PublicEthereumAPI) pendingMsgs() ([]sdk.Msg, error) {
	// nolint: prealloc
	var msgs []sdk.Msg

	pendingTxs, err := api.PendingTransactions()
	if err != nil {
		return nil, err
	}

	for _, pendingTx := range pendingTxs {
		// NOTE: we have to construct the EVM transaction instead of just casting from the tendermint
		// transactions because PendingTransactions only checks for MsgEthereumTx messages.

		pendingTo := sdk.AccAddress(pendingTx.To.Bytes())
		pendingFrom := sdk.AccAddress(pendingTx.From.Bytes())
		pendingGas, err := hexutil.DecodeUint64(pendingTx.Gas.String())
		if err != nil {
			return nil, err
		}

		pendingValue := pendingTx.Value.ToInt()
		pendingGasPrice := new(big.Int).SetUint64(evmtypes.DefaultGasPrice)
		if pendingTx.GasPrice != nil {
			pendingGasPrice = pendingTx.GasPrice.ToInt()
		}

		pendingData := pendingTx.Input
		nonce, _ := api.accountNonce(api.clientCtx, pendingTx.From, true)

		msg := evmtypes.NewMsgEthermint(nonce, &pendingTo, sdk.NewIntFromBigInt(pendingValue), pendingGas,
			sdk.NewIntFromBigInt(pendingGasPrice), pendingData, pendingFrom)

		msgs = append(msgs, msg)
	}
	return msgs, nil
}

// accountNonce returns looks up the transaction nonce count for a given address. If the pending boolean
// is set to true, it will add to the counter all the uncommitted EVM transactions sent from the address.
// NOTE: The function returns no error if the account doesn't exist.
func (api *PublicEthereumAPI) accountNonce(
	clientCtx client.Context, address common.Address, pending bool,
) (uint64, error) {
	// Get nonce (sequence) from sender account
	from := sdk.AccAddress(address.Bytes())

	// use a the given client context in case its wrapped with a custom height
	accRet := clientCtx.AccountRetriever

	if err := accRet.EnsureExists(api.clientCtx, from); err != nil {
		// account doesn't exist yet, return 0
		return 0, nil
	}

	_, nonce, err := accRet.GetAccountNumberSequence(api.clientCtx, from)
	if err != nil {
		return 0, err
	}

	if !pending {
		return nonce, nil
	}

	// the account retriever doesn't include the uncommitted transactions on the nonce so we need to
	// to manually add them.
	pendingTxs, err := api.backend.UserPendingTransactionsCnt(address.String())
	if err != nil {
		return 0, err
	}
	nonce += uint64(pendingTxs)

	return nonce, nil
}
