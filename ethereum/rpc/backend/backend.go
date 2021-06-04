package backend

import (
	"context"
	"fmt"
	"golang.org/x/time/rate"

	"github.com/tendermint/tendermint/libs/log"
	rpctypes "github.com/cosmos/gaia/v4/ethereum/rpc/types"
	evmtypes "github.com/cosmos/cosmos-sdk/x/evm/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/bitutil"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/bloombits"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

// Backend implements the functionality needed to filter changes.
// Implemented by EthermintBackend.
type Backend interface {
	// Used by block filter; also used for polling
	BlockNumber() (hexutil.Uint64, error)
	LatestBlockNumber() (int64, error)
	HeaderByNumber(blockNum rpctypes.BlockNumber) (*ethtypes.Header, error)
	HeaderByHash(blockHash common.Hash) (*ethtypes.Header, error)
	GetBlockByNumber(blockNum rpctypes.BlockNumber, fullTx bool) (interface{}, error)
	GetBlockByHash(hash common.Hash, fullTx bool) (interface{}, error)

	// returns the logs of a given block
	GetLogs(blockHash common.Hash) ([][]*ethtypes.Log, error)

	// Used by pending transaction filter
	PendingTransactions() ([]*rpctypes.Transaction, error)
	PendingTransactionCnt() (int, error)
	PendingTransactionsByHash(target common.Hash) (*rpctypes.Transaction, error)
	UserPendingTransactionsCnt(address string) (int, error)
	UserPendingTransactions(address string, limit int) ([]*rpctypes.Transaction, error)

	// Used by log filter
	GetTransactionLogs(txHash common.Hash) ([]*ethtypes.Log, error)
	BloomStatus() (uint64, uint64)
	ServiceFilter(ctx context.Context, session *bloombits.MatcherSession)
}

var _ Backend = (*EthermintBackend)(nil)

// EthermintBackend implements the Backend interface
type EthermintBackend struct {
	ctx               context.Context
	clientCtx         client.Context
	queryClient evmtypes.QueryClient // gRPC query client
	logger            log.Logger
	gasLimit          int64
	bloomRequests     chan chan *bloombits.Retrieval
	closeBloomHandler chan struct{}
	rateLimiters      map[string]*rate.Limiter
}

// New creates a new EthermintBackend instance
func New(clientCtx client.Context, log log.Logger, rateLimiters map[string]*rate.Limiter) *EthermintBackend {
	return &EthermintBackend{
		ctx:               context.Background(),
		clientCtx:         clientCtx,
		logger:            log.With("module", "json-rpc"),
		gasLimit:          int64(^uint32(0)),
		bloomRequests:     make(chan chan *bloombits.Retrieval),
		closeBloomHandler: make(chan struct{}),
		rateLimiters:      rateLimiters,

		queryClient: evmtypes.NewQueryClient(clientCtx),
	}
}

// BlockNumber returns the current block number.
func (b *EthermintBackend) BlockNumber() (hexutil.Uint64, error) {
	blockNumber, err := b.LatestBlockNumber()
	if err != nil {
		return hexutil.Uint64(0), err
	}

	if blockNumber > 0 {
		//decrease blockNumber to make sure every block has been executed in local
		blockNumber--
	}
	return hexutil.Uint64(blockNumber), nil
}

// GetBlockByNumber returns the block identified by number.
func (b *EthermintBackend) GetBlockByNumber(blockNum rpctypes.BlockNumber, fullTx bool) (interface{}, error) {
	height := blockNum.Int64()
	if height <= 0 {
		// get latest block height
		num, err := b.BlockNumber()
		if err != nil {
			return nil, err
		}

		height = int64(num)
	}

	resBlock, err := b.clientCtx.Client.Block(b.ctx,&height)
	if err != nil {
		return nil, nil
	}

	return rpctypes.EthBlockFromTendermint(b.clientCtx, resBlock.Block, fullTx)
}

// GetBlockByHash returns the block identified by hash.
func (b *EthermintBackend) GetBlockByHash(hash common.Hash, fullTx bool) (interface{}, error) {
	res, _, err := b.clientCtx.Query(fmt.Sprintf("custom/%s/%s/%s", evmtypes.ModuleName, evmtypes.QueryHashToHeight, hash.Hex()))
	if err != nil {
		return nil, err
	}

	var out evmtypes.QueryResBlockNumber
	if err := b.clientCtx.Codec.UnmarshalJSON(res, &out); err != nil {
		return nil, err
	}

	resBlock, err := b.clientCtx.Client.Block(b.ctx,&out.Number)
	if err != nil {
		return nil, nil
	}

	return rpctypes.EthBlockFromTendermint(b.clientCtx, resBlock.Block, fullTx)
}

// HeaderByNumber returns the block header identified by height.
func (b *EthermintBackend) HeaderByNumber(blockNum rpctypes.BlockNumber) (*ethtypes.Header, error) {
	height := blockNum.Int64()
	if height <= 0 {
		// get latest block height
		num, err := b.BlockNumber()
		if err != nil {
			return nil, err
		}

		height = int64(num)
	}

	resBlock, err := b.clientCtx.Client.Block(b.ctx,&height)
	if err != nil {
		return nil, err
	}

	res, _, err := b.clientCtx.Query(fmt.Sprintf("custom/%s/%s/%d", evmtypes.ModuleName, evmtypes.QueryBloom, resBlock.Block.Height))
	if err != nil {
		return nil, err
	}

	var bloomRes evmtypes.QueryBloomFilter
	b.clientCtx.Codec.MustUnmarshalJSON(res, &bloomRes)

	ethHeader := rpctypes.EthHeaderFromTendermint(resBlock.Block.Header)
	ethHeader.Bloom = bloomRes.Bloom
	return ethHeader, nil
}

// HeaderByHash returns the block header identified by hash.
func (b *EthermintBackend) HeaderByHash(blockHash common.Hash) (*ethtypes.Header, error) {
	res, _, err := b.clientCtx.Query(fmt.Sprintf("custom/%s/%s/%s", evmtypes.ModuleName, evmtypes.QueryHashToHeight, blockHash.Hex()))
	if err != nil {
		return nil, err
	}

	var out evmtypes.QueryResBlockNumber
	if err := b.clientCtx.Codec.UnmarshalJSON(res, &out); err != nil {
		return nil, err
	}

	resBlock, err := b.clientCtx.Client.Block(b.ctx,&out.Number)
	if err != nil {
		return nil, err
	}

	res, _, err = b.clientCtx.Query(fmt.Sprintf("custom/%s/%s/%d", evmtypes.ModuleName, evmtypes.QueryBloom, resBlock.Block.Height))
	if err != nil {
		return nil, err
	}

	var bloomRes evmtypes.QueryBloomFilter
	b.clientCtx.Codec.MustUnmarshalJSON(res, &bloomRes)

	ethHeader := rpctypes.EthHeaderFromTendermint(resBlock.Block.Header)
	ethHeader.Bloom = bloomRes.Bloom
	return ethHeader, nil
}

// GetTransactionLogs returns the logs given a transaction hash.
// It returns an error if there's an encoding error.
// If no logs are found for the tx hash, the error is nil.
func (b *EthermintBackend) GetTransactionLogs(txHash common.Hash) ([]*ethtypes.Log, error) {
	txRes, err := b.clientCtx.Client.Tx(txHash.Bytes(), !b.clientCtx.TrustNode)
	if err != nil {
		return nil, err
	}

	execRes, err := evmtypes.DecodeResultData(txRes.TxResult.Data)
	if err != nil {
		return nil, err
	}

	return execRes.Logs, nil
}

// PendingTransactions returns the transactions that are in the transaction pool
// and have a from address that is one of the accounts this node manages.
func (b *EthermintBackend) PendingTransactions() ([]*rpctypes.Transaction, error) {
	pendingTxs, err := b.clientCtx.Client.UnconfirmedTxs(-1)
	if err != nil {
		return nil, err
	}

	transactions := make([]*rpctypes.Transaction, 0)
	for _, tx := range pendingTxs.Txs {
		ethTx, err := rpctypes.RawTxToEthTx(b.clientCtx, tx)
		if err != nil {
			// ignore non Ethermint EVM transactions
			continue
		}

		// TODO: check signer and reference against accounts the node manages
		rpcTx, err := rpctypes.NewTransaction(ethTx, common.BytesToHash(tx.Hash()), common.Hash{}, 0, 0)
		if err != nil {
			return nil, err
		}

		transactions = append(transactions, rpcTx)
	}

	return transactions, nil
}

func (b *EthermintBackend) PendingTransactionCnt() (int, error) {
	result, err := b.clientCtx.Client.UnconfirmedTxs(b.ctx,-1)
	if err != nil {
		return 0, err
	}
	return result.Count, nil
}

func (b *EthermintBackend) UserPendingTransactionsCnt(address string) (int, error) {
	result, err := b.clientCtx.Client.UserNumUnconfirmedTxs(address)
	if err != nil {
		return 0, err
	}
	return result.Count, nil
}

func (b *EthermintBackend) UserPendingTransactions(address string, limit int) ([]*rpctypes.Transaction, error) {
	result, err := b.clientCtx.Client.UserUnconfirmedTxs(address, limit)
	if err != nil {
		return nil, err
	}

	transactions := make([]*rpctypes.Transaction, len(result.Txs))
	for _, tx := range result.Txs {
		ethTx, err := rpctypes.RawTxToEthTx(b.clientCtx, tx)
		if err != nil {
			// ignore non Ethermint EVM transactions
			continue
		}

		// TODO: check signer and reference against accounts the node manages
		rpcTx, err := rpctypes.NewTransaction(ethTx, common.BytesToHash(tx.Hash()), common.Hash{}, 0, 0)
		if err != nil {
			return nil, err
		}

		transactions = append(transactions, rpcTx)
	}

	return transactions, nil
}

// PendingTransactions returns the transaction that is in the transaction pool
// and have a from address that is one of the accounts this node manages.
func (b *EthermintBackend) PendingTransactionsByHash(target common.Hash) (*rpctypes.Transaction, error) {
	pendingTx, err := b.clientCtx.Client.GetUnconfirmedTxByHash(target)
	if err != nil {
		return nil, err
	}
	ethTx, err := rpctypes.RawTxToEthTx(b.clientCtx, pendingTx)
	if err != nil {
		// ignore non Ethermint EVM transactions
		return nil, err
	}
	rpcTx, err := rpctypes.NewTransaction(ethTx, common.BytesToHash(pendingTx.Hash()), common.Hash{}, 0, 0)
	if err != nil {
		return nil, err
	}
	return rpcTx, nil
}

// GetLogs returns all the logs from all the ethereum transactions in a block.
func (b *EthermintBackend) GetLogs(blockHash common.Hash) ([][]*ethtypes.Log, error) {
	res, _, err := b.clientCtx.Query(fmt.Sprintf("custom/%s/%s/%s", evmtypes.ModuleName, evmtypes.QueryHashToHeight, blockHash.Hex()))
	if err != nil {
		return nil, err
	}

	var out evmtypes.QueryResBlockNumber
	if err := b.clientCtx.Codec.UnmarshalJSON(res, &out); err != nil {
		return nil, err
	}

	block, err := b.clientCtx.Client.Block(b.ctx,&out.Number)
	if err != nil {
		return nil, err
	}

	var blockLogs = [][]*ethtypes.Log{}
	for _, tx := range block.Block.Txs {
		// NOTE: we query the state in case the tx result logs are not persisted after an upgrade.
		txRes, err := b.clientCtx.Client.Tx(tx.Hash(), !b.clientCtx.TrustNode)
		if err != nil {
			continue
		}
		execRes, err := evmtypes.DecodeResultData(txRes.TxResult.Data)
		if err != nil {
			continue
		}

		blockLogs = append(blockLogs, execRes.Logs)
	}

	return blockLogs, nil
}

// BloomStatus returns the BloomBitsBlocks and the number of processed sections maintained
// by the chain indexer.
func (b *EthermintBackend) BloomStatus() (uint64, uint64) {
	sections := evmtypes.GetIndexer().StoredSection()
	return evmtypes.BloomBitsBlocks, sections
}

// LatestBlockNumber gets the latest block height in int64 format.
func (b *EthermintBackend) LatestBlockNumber() (int64, error) {
	// NOTE: using 0 as min and max height returns the blockchain info up to the latest block.
	info, err := b.clientCtx.Client.BlockchainInfo(b.ctx,0, 0)
	if err != nil {
		return 0, err
	}

	return info.LastHeight, nil
}

func (b *EthermintBackend) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	for i := 0; i < evmtypes.BloomFilterThreads; i++ {
		go session.Multiplex(evmtypes.BloomRetrievalBatch, evmtypes.BloomRetrievalWait, b.bloomRequests)
	}
}

// startBloomHandlers starts a batch of goroutines to accept bloom bit database
// retrievals from possibly a range of filters and serving the data to satisfy.
func (b *EthermintBackend) StartBloomHandlers(sectionSize uint64, db dbm.DB) {
	for i := 0; i < evmtypes.BloomServiceThreads; i++ {
		go func() {
			for {
				select {
				case <-b.closeBloomHandler:
					return

				case request := <-b.bloomRequests:
					task := <-request
					task.Bitsets = make([][]byte, len(task.Sections))
					for i, section := range task.Sections {
						height := int64((section+1)*sectionSize-1) + tmtypes.GetStartBlockHeight()
						hash, err := b.GetBlockHashByHeight(rpctypes.BlockNumber(height))
						if err != nil {
							task.Error = err
						}
						if compVector, err := evmtypes.ReadBloomBits(db, task.Bit, section, hash); err == nil {
							if blob, err := bitutil.DecompressBytes(compVector, int(sectionSize/8)); err == nil {
								task.Bitsets[i] = blob
							} else {
								task.Error = err
							}
						} else {
							task.Error = err
						}
					}
					request <- task
				}
			}
		}()
	}
}

// GetBlockHashByHeight returns the block hash by height.
func (b *EthermintBackend) GetBlockHashByHeight(height rpctypes.BlockNumber) (common.Hash, error) {
	res, _, err := b.clientCtx.Query(fmt.Sprintf("custom/%s/%s/%d", evmtypes.ModuleName, evmtypes.QueryHeightToHash, height))
	if err != nil {
		return common.Hash{}, err
	}

	hash := common.BytesToHash(res)
	return hash, nil
}

// Close
func (b *EthermintBackend) Close() {
	close(b.closeBloomHandler)
}

func (b *EthermintBackend) GetRateLimiter(apiName string) *rate.Limiter {
	if b.rateLimiters == nil {
		return nil
	}
	return b.rateLimiters[apiName]
}
