package types

import (
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var (
	ErrInvalidState           = sdkerrors.Register(ModuleName, 2, "invalid storage state")
	ErrChainConfigNotFound    = sdkerrors.Register(ModuleName, 3, "chain configuration not found")
	ErrInvalidChainConfig     = sdkerrors.Register(ModuleName, 4, "invalid chain configuration")
	ErrCreateDisabled         = sdkerrors.Register(ModuleName, 5, "EVM Create operation is disabled")
	ErrCallDisabled           = sdkerrors.Register(ModuleName, 6, "EVM Call operation is disabled")
	ErrKeyNotFound            = sdkerrors.Register(ModuleName, 8, "Key not found in database")
	ErrStrConvertFailed       = sdkerrors.Register(ModuleName, 9, "Failed to convert string")
	ErrUnexpectedProposalType = sdkerrors.Register(ModuleName, 10, "Unsupported proposal type of evm module")
	ErrEmptyAddressList       = sdkerrors.Register(ModuleName, 11, "Empty account address list")
	ErrDuplicatedAddr         = sdkerrors.Register(ModuleName, 12, "Duplicated address in address list")
	ErrInvalidValue           = sdkerrors.Register(ModuleName, 13, "invalid value")
	ErrInvalidChainID         = sdkerrors.Register(ModuleName, 14, "invalid chain ID")
	ErrVMExecution            = sdkerrors.Register(ModuleName, 15, "error while executing evm transaction")
	ErrInvalidMsgType         = sdkerrors.Register(ModuleName, 16, "invalid message type")
	ErrCallBlockedContract    = sdkerrors.Register(ModuleName, 17, "invalid contract")
	ErrUnauthorizedAccount    = sdkerrors.Register(ModuleName, 18, "invalid contract")
	CodeSpaceEvmCallFailed    = uint32(7)
	ErrorHexData              = "HexData"
)
