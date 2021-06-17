package keeper

import (
	"encoding/binary"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethcmn "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/spf13/viper"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gaia/v4/x/evm/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
)

// Keeper wraps the CommitStateDB, allowing us to pass in SDK context while adhering
// to the StateDB interface.
type Keeper struct {
	// Amino codec
	cdc codec.BinaryMarshaler
	// Store key required for the EVM Prefix KVStore. It is required by:
	// - storing Account's Storage State
	// - storing Account's Code
	// - storing transaction Logs
	// - storing block height -> bloom filter map. Needed for the Web3 API.
	// - storing block hash -> block height map. Needed for the Web3 API.
	storeKey   sdk.StoreKey
	paramSpace paramtypes.Subspace

	accountKeeper types.AccountKeeper
	bankKeeper    types.BankKeeper
	govKeeper     types.GovKeeper

	// Transaction counter in a block. Used on StateSB's Prepare function.
	// It is reset to 0 every block on BeginBlock so there's no point in storing the counter
	// on the KVStore or adding it as a field on the EVM genesis state.
	TxCount int
	Bloom   *big.Int
	Bhash   ethcmn.Hash
	LogSize uint
}

// NewKeeper creates a new evm Keeper instance
func NewKeeper(
	cdc codec.BinaryMarshaler, key sdk.StoreKey, ak types.AccountKeeper, bk types.BankKeeper, gk types.GovKeeper,
	ps paramtypes.Subspace,
) Keeper {
	// set KeyTable if it has not already been set
	if !ps.HasKeyTable() {
		ps = ps.WithKeyTable(types.ParamKeyTable())
	}

	if enable := viper.GetBool(types.FlagEnableBloomFilter); enable {
		types.SetEnableBloomFilter(enable)
		db := types.BloomDb()
		types.InitIndexer(db)
	}

	return Keeper{
		cdc:           cdc,
		storeKey:      key,
		paramSpace:    ps,
		accountKeeper: ak,
		bankKeeper:    bk,
		govKeeper:     gk,
		TxCount:       0,
		Bloom:         big.NewInt(0),
		LogSize:       0,
	}
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", "x/"+types.ModuleName)
}

func (k Keeper) GenerateCSDBParams() types.CommitStateDBParams {
	return types.CommitStateDBParams{
		StoreKey:      k.storeKey,
		ParamSpace:    k.paramSpace,
		AccountKeeper: k.accountKeeper,
		BankKeeper:    k.bankKeeper,
	}
}

// GeneratePureCSDBParams generates an instance of csdb params ONLY for store setter and getter
func (k Keeper) GeneratePureCSDBParams() types.CommitStateDBParams {
	return types.CommitStateDBParams{
		StoreKey: k.storeKey,
	}
}

// GetChainConfig gets block height from block consensus hash
func (k Keeper) GetChainConfig(ctx sdk.Context) (types.ChainConfig, bool) {
	store := ctx.KVStore(k.storeKey)

	bz := store.Get(types.KeyPrefixChainConfig)
	if len(bz) == 0 {
		return types.ChainConfig{}, false
	}

	var config types.ChainConfig
	k.cdc.MustUnmarshalBinaryBare(bz, &config)
	return config, true
}

// SetChainConfig sets the mapping from block consensus hash to block height
func (k Keeper) SetChainConfig(ctx sdk.Context, config types.ChainConfig) {
	store := ctx.KVStore(k.storeKey)
	bz := k.cdc.MustMarshalBinaryBare(&config)
	store.Set(types.KeyPrefixChainConfig, bz)
}

// ----------------------------------------------------------------------------
// Block hash mapping functions
// Required by Web3 API.
//  TODO: remove once tendermint support block queries by hash.
// ----------------------------------------------------------------------------

// GetBlockHash gets block height from block consensus hash
func (k Keeper) GetBlockHash(ctx sdk.Context, hash []byte) (int64, bool) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixBlockHash)
	bz := store.Get(hash)
	if len(bz) == 0 {
		return 0, false
	}

	height := binary.BigEndian.Uint64(bz)
	return int64(height), true
}

// SetBlockHash sets the mapping from block consensus hash to block height
func (k Keeper) SetBlockHash(ctx sdk.Context, hash []byte, height int64) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixBlockHash)
	bz := sdk.Uint64ToBigEndian(uint64(height))
	store.Set(hash, bz)
}

// ----------------------------------------------------------------------------
// Epoch Height -> hash mapping functions
// Required by EVM context's GetHashFunc
// ----------------------------------------------------------------------------

// GetHeightHash returns the block header hash associated with a given block height and chain epoch number.
func (k Keeper) GetHeightHash(ctx sdk.Context, height uint64) ethcmn.Hash {
	return types.CreateEmptyCommitStateDB(k.GenerateCSDBParams(), ctx).GetHeightHash(height)
}

// SetHeightHash sets the block header hash associated with a given height.
func (k Keeper) SetHeightHash(ctx sdk.Context, height uint64, hash ethcmn.Hash) {
	types.CreateEmptyCommitStateDB(k.GenerateCSDBParams(), ctx).SetHeightHash(height, hash)
}

// ----------------------------------------------------------------------------
// Block bloom bits mapping functions
// Required by Web3 API.
// ----------------------------------------------------------------------------

// GetBlockBloom gets bloombits from block height
func (k Keeper) GetBlockBloom(ctx sdk.Context, height int64) ethtypes.Bloom {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixBloom)
	has := store.Has(types.BloomKey(height))
	if !has {
		return ethtypes.Bloom{}
	}

	bz := store.Get(types.BloomKey(height))
	return ethtypes.BytesToBloom(bz)
}

// SetBlockBloom sets the mapping from block height to bloom bits
func (k Keeper) SetBlockBloom(ctx sdk.Context, height int64, bloom ethtypes.Bloom) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixBloom)
	store.Set(types.BloomKey(height), bloom.Bytes())
}

// GetAccountStorage return state storage associated with an account
func (k Keeper) GetAccountStorage(ctx sdk.Context, address ethcmn.Address) (types.Storage, error) {
	storage := types.Storage{}
	csdb := types.CreateEmptyCommitStateDB(k.GenerateCSDBParams(), ctx)
	err := csdb.ForEachStorage(address, func(key, value common.Hash) bool {
		storage = append(storage, types.NewState(key, value))
		return false
	})
	if err != nil {
		return types.Storage{}, err
	}

	return storage, nil
}
