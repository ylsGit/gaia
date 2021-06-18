package keeper

import (
	"encoding/json"
	"fmt"
	"strconv"

	ethcmn "github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	abci "github.com/tendermint/tendermint/abci/types"

	"github.com/cosmos/gaia/v4/x/evm/types"
)

// NewQuerier creates a querier for evm REST endpoints
func NewQuerier(k Keeper, legacyQuerierCdc *codec.LegacyAmino) sdk.Querier {
	return func(ctx sdk.Context, path []string, _ abci.RequestQuery) ([]byte, error) {
		if len(path) < 1 {
			return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidRequest,
				"Insufficient parameters, at least 1 parameter is required")
		}

		switch path[0] {
		case types.QueryBalance:
			return queryBalance(ctx, path, k, legacyQuerierCdc)
		//case types.QueryBlockNumber:
		//	return queryBlockNumber(ctx, legacyQuerierCdc)
		case types.QueryStorage:
			return queryStorage(ctx, path, k, legacyQuerierCdc)
		case types.QueryCode:
			return queryCode(ctx, path, k, legacyQuerierCdc)
		case types.QueryHashToHeight:
			return queryHashToHeight(ctx, path, k, legacyQuerierCdc)
		case types.QueryBloom:
			return queryBlockBloom(ctx, path, k, legacyQuerierCdc)
		case types.QueryAccount:
			return queryAccount(ctx, path, k, legacyQuerierCdc)
		//case types.QueryExportAccount:
		//	return queryExportAccount(ctx, path, k)
		case types.QueryParameters:
			return queryParams(ctx, k, legacyQuerierCdc)
		case types.QueryHeightToHash:
			return queryHeightToHash(ctx, path, k)
		case types.QuerySection:
			return querySection(path)
		default:
			return nil, sdkerrors.Wrapf(sdkerrors.ErrUnknownRequest, "unknown %s query endpoint: %s", types.ModuleName, path[0])
		}
	}
}

func queryBalance(ctx sdk.Context, path []string, k Keeper, legacyQuerierCdc *codec.LegacyAmino) ([]byte, error) {
	if len(path) < 2 {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidRequest,
			"Insufficient parameters, at least 2 parameters is required")
	}

	addr := ethcmn.HexToAddress(path[1])
	balance := k.GetBalance(ctx, addr)
	balanceStr, err := types.MarshalBigInt(balance)
	if err != nil {
		return nil, err
	}

	res := types.QueryResBalance{Balance: balanceStr}
	bz, err := codec.MarshalJSONIndent(legacyQuerierCdc, res)
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
	}

	return bz, nil
}

//func queryBlockNumber(ctx sdk.Context, legacyQuerierCdc *codec.LegacyAmino) ([]byte, error) {
//	num := ctx.BlockHeight()
//	bnRes := types.QueryResBlockNumber{Number: num}
//	bz, err := codec.MarshalJSONIndent(legacyQuerierCdc, bnRes)
//	if err != nil {
//		return nil, sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
//	}
//
//	return bz, nil
//}

func queryStorage(ctx sdk.Context, path []string, k Keeper, legacyQuerierCdc *codec.LegacyAmino) ([]byte, error) {
	if len(path) < 3 {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidRequest,
			"Insufficient parameters, at least 3 parameters is required")
	}

	addr := ethcmn.HexToAddress(path[1])
	key := ethcmn.HexToHash(path[2])
	val := k.GetState(ctx, addr, key)
	res := types.QueryResStorage{Value: val.Bytes()}
	bz, err := codec.MarshalJSONIndent(legacyQuerierCdc, res)
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
	}
	return bz, nil
}

func queryCode(ctx sdk.Context, path []string, k Keeper, legacyQuerierCdc *codec.LegacyAmino) ([]byte, error) {
	if len(path) < 2 {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidRequest,
			"Insufficient parameters, at least 2 parameters is required")
	}

	addr := ethcmn.HexToAddress(path[1])
	code := k.GetCode(ctx, addr)
	res := types.QueryResCode{Code: code}
	bz, err := codec.MarshalJSONIndent(legacyQuerierCdc, res)
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
	}

	return bz, nil
}

func queryHashToHeight(ctx sdk.Context, path []string, k Keeper, legacyQuerierCdc *codec.LegacyAmino) ([]byte, error) {
	if len(path) < 2 {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidRequest,
			"Insufficient parameters, at least 2 parameters is required")
	}

	blockHash := ethcmn.FromHex(path[1])
	blockNumber, found := k.GetBlockHash(ctx, blockHash)
	if !found {
		return []byte{}, sdkerrors.Wrap(types.ErrKeyNotFound, fmt.Sprintf("block height not found for hash %s", path[1]))
	}

	res := types.QueryResBlockNumber{Number: blockNumber}
	bz, err := codec.MarshalJSONIndent(legacyQuerierCdc, res)
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
	}

	return bz, nil
}

func queryBlockBloom(ctx sdk.Context, path []string, k Keeper, legacyQuerierCdc *codec.LegacyAmino) ([]byte, error) {
	if len(path) < 2 {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidRequest,
			"Insufficient parameters, at least 2 parameters is required")
	}

	num, err := strconv.ParseInt(path[1], 10, 64)
	if err != nil {
		return nil, sdkerrors.Wrap(types.ErrStrConvertFailed, fmt.Sprintf("could not unmarshal block height: %s", err))
	}

	bloom := k.GetBlockBloom(ctx.WithBlockHeight(num), num)
	res := types.QueryBloomFilter{Bloom: bloom}
	bz, err := codec.MarshalJSONIndent(legacyQuerierCdc, res)
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
	}

	return bz, nil
}

func queryAccount(ctx sdk.Context, path []string, k Keeper, legacyQuerierCdc *codec.LegacyAmino) ([]byte, error) {
	if len(path) < 2 {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidRequest,
			"Insufficient parameters, at least 2 parameters is required")
	}

	addr := ethcmn.HexToAddress(path[1])
	so := k.GetOrNewStateObject(ctx, addr)

	balance, err := types.MarshalBigInt(so.Balance())
	if err != nil {
		return nil, err
	}

	res := types.QueryResAccount{
		Balance:  balance,
		CodeHash: so.CodeHash(),
		Nonce:    so.Nonce(),
	}
	bz, err := codec.MarshalJSONIndent(legacyQuerierCdc, res)
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
	}
	return bz, nil
}

//
//func queryExportAccount(ctx sdk.Context, path []string, k Keeper) ([]byte, error) {
//	if len(path) < 2 {
//		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidRequest,
//			"Insufficient parameters, at least 2 parameters is required")
//	}
//
//	hexAddress := path[1]
//	addr := ethcmn.HexToAddress(hexAddress)
//
//	var storage types.Storage
//	err := k.ForEachStorage(ctx, addr, func(key, value ethcmn.Hash) bool {
//		storage = append(storage, types.NewState(key, value))
//		return false
//	})
//	if err != nil {
//		return nil, err
//	}
//
//	res := types.GenesisAccount{
//		Address: hexAddress,
//		Code:    k.GetCode(ctx, addr),
//		Storage: storage,
//	}
//
//	// TODO: codec.MarshalJSONIndent doesn't call the String() method of types properly
//	bz, err := json.MarshalIndent(res, "", "\t")
//	if err != nil {
//		return nil, sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
//	}
//
//	return bz, nil
//}

func queryParams(ctx sdk.Context, k Keeper, legacyQuerierCdc *codec.LegacyAmino) ([]byte, error) {
	params := k.GetParams(ctx)
	res, err := codec.MarshalJSONIndent(legacyQuerierCdc, params)
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
	}
	return res, nil
}

func queryHeightToHash(ctx sdk.Context, path []string, k Keeper) ([]byte, error) {
	if len(path) < 2 {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidRequest,
			"Insufficient parameters, at least 2 parameters is required")
	}

	height, err := strconv.Atoi(path[1])
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidRequest,
			"Insufficient parameters, params[1] convert to int failed")
	}
	hash := k.GetHeightHash(ctx, uint64(height))

	return hash.Bytes(), nil
}

func querySection(path []string) ([]byte, error) {
	if !types.GetEnableBloomFilter() {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidRequest,
			"disable bloom filter")
	}

	if len(path) != 1 {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidRequest,
			"wrong parameters, need no parameters")
	}

	res, err := json.Marshal(types.GetIndexer().StoredSection())
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
	}

	return res, nil
}
