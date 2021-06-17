package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	ethcmn "github.com/ethereum/go-ethereum/common"
)

const (
	// ModuleName is the name of the evm module
	ModuleName = "evm"

	// StoreKey is the string store representation
	StoreKey = ModuleName

	// QuerierRoute is the querier route for the evm module
	QuerierRoute = ModuleName

	// RouterKey is the msg router key for the evm module
	RouterKey = ModuleName
)

var (
	KeyPrefixBlockHash                   = []byte{0x01}
	KeyPrefixBloom                       = []byte{0x02}
	KeyPrefixCode                        = []byte{0x04}
	KeyPrefixStorage                     = []byte{0x05}
	KeyPrefixChainConfig                 = []byte{0x06}
	KeyPrefixHeightHash                  = []byte{0x07}
	KeyPrefixContractDeploymentWhitelist = []byte{0x08}
	KeyPrefixContractBlockedList         = []byte{0x09}
)

// HeightHashKey returns the key for the given chain epoch and height.
// The key will be composed in the following order:
//   key = prefix + bytes(height)
// This ordering facilitates the iteration by height for the EVM GetHashFn
// queries.
func HeightHashKey(height uint64) []byte {
	return sdk.Uint64ToBigEndian(height)
}

// BloomKey defines the store key for a block Bloom
func BloomKey(height int64) []byte {
	return sdk.Uint64ToBigEndian(uint64(height))
}

// AddressStoragePrefix returns a prefix to iterate over a given account storage.
func AddressStoragePrefix(address ethcmn.Address) []byte {
	return append(KeyPrefixStorage, address.Bytes()...)
}

// getContractDeploymentWhitelistMemberKey builds the key for an approved contract deployer
func getContractDeploymentWhitelistMemberKey(distributorAddr sdk.AccAddress) []byte {
	return append(KeyPrefixContractDeploymentWhitelist, distributorAddr...)
}

// splitApprovedDeployerAddress splits the deployer address from a ContractDeploymentWhitelistMemberKey
func splitApprovedDeployerAddress(key []byte) sdk.AccAddress {
	return key[1:]
}

// getContractBlockedListMemberKey builds the key for a blocked contract address
func getContractBlockedListMemberKey(contractAddr sdk.AccAddress) []byte {
	return append(KeyPrefixContractBlockedList, contractAddr...)
}

// splitBlockedContractAddress splits the blocked contract address from a ContractBlockedListMemberKey
func splitBlockedContractAddress(key []byte) sdk.AccAddress {
	return key[1:]
}
