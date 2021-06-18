package keeper

import (
	"context"

	ethcmn "github.com/ethereum/go-ethereum/common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gaia/v4/x/evm/types"
)

var _ types.QueryServer = Keeper{}

// Balance queries the balance of all coins for a single account.
func (k Keeper) Balance(c context.Context, req *types.QueryBalanceRequest) (*types.QueryBalanceResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "empty request")
	}

	if req.Address == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(c)
	balance := k.GetBalance(ctx, ethcmn.HexToAddress(req.Address))
	balanceStr, err := types.MarshalBigInt(balance)
	if err != nil {
		return nil, err
	}

	return &types.QueryBalanceResponse{Balance: balanceStr}, nil
}

//
func (k Keeper) Storage(c context.Context, req *types.QueryStorageRequest) (*types.QueryStorageResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "empty request")
	}

	if req.Address == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request")
	}

	if req.Key == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(c)
	val := k.GetState(ctx, ethcmn.HexToAddress(req.Address), ethcmn.HexToHash(req.Key))

	return &types.QueryStorageResponse{Value: val.Bytes()}, nil
}

//
func (k Keeper) Code(c context.Context, req *types.QueryCodeRequest) (*types.QueryCodeResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "empty request")
	}

	if req.Address == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(c)
	code := k.GetCode(ctx, ethcmn.HexToAddress(req.Address))

	return &types.QueryCodeResponse{Code: code}, nil
}

//
func (k Keeper) BlockNumber(context.Context, *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	return nil, nil
}

//
func (k Keeper) HashToHeight(c context.Context, req *types.QueryHeightRequest) (*types.QueryHeightResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "empty request")
	}

	if len(req.Hash) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(c)
	blockNumber, found := k.GetBlockHash(ctx, req.Hash)
	if !found {
		return nil, status.Errorf(codes.NotFound, "Block height not found for hash %s", ethcmn.BytesToHash(req.Hash))
	}

	return &types.QueryHeightResponse{Height: blockNumber}, nil

}

//
func (k Keeper) HeightToHash(c context.Context, req *types.QueryHashRequest) (*types.QueryHashResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "empty request")
	}

	ctx := sdk.UnwrapSDKContext(c)
	hash := k.GetHeightHash(ctx, uint64(req.Height))

	return &types.QueryHashResponse{Hash: hash.Bytes()}, nil
}

// BlockBloom implements the Query/BlockBloom gRPC method
func (k Keeper) BlockBloom(c context.Context, req *types.QueryBlockBloomRequest) (*types.QueryBlockBloomResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "empty request")
	}

	ctx := sdk.UnwrapSDKContext(c)
	bloom := k.GetBlockBloom(ctx.WithBlockHeight(req.Height), req.Height)

	return &types.QueryBlockBloomResponse{Bloom: bloom.Bytes()}, nil
}

// Account implements the Query/Account gRPC method
func (k Keeper) Account(c context.Context, req *types.QueryAccountRequest) (*types.QueryAccountResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if err := types.ValidateAddress(req.Address); err != nil {
		return nil, status.Error(
			codes.InvalidArgument, err.Error(),
		)
	}

	ctx := sdk.UnwrapSDKContext(c)

	so := k.GetOrNewStateObject(ctx, ethcmn.HexToAddress(req.Address))
	balance, err := types.MarshalBigInt(so.Balance())
	if err != nil {
		return nil, err
	}

	return &types.QueryAccountResponse{
		Balance:  balance,
		CodeHash: so.CodeHash(),
		Nonce:    so.Nonce(),
	}, nil
}

//
func (k Keeper) ExportAccount(context.Context, *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	return nil, nil
}

// Params queries the parameters of x/evm module.
func (k Keeper) Params(c context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	params := k.GetParams(ctx)

	return &types.QueryParamsResponse{Params: params}, nil
}

// Section queries the parameters of x/evm module.
func (k Keeper) Section(context.Context, *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	return nil, nil
}

// ContractDeploymentWhitelist queries the parameters of x/evm module.
func (k Keeper) ContractDeploymentWhitelist(context.Context, *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	return nil, nil
}

// ContractBlockedList queries the parameters of x/evm module.
func (k Keeper) ContractBlockedList(context.Context, *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	return nil, nil
}
