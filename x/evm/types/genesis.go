package types

import (
	"errors"
	"fmt"

	ethcmn "github.com/ethereum/go-ethereum/common"
)

// NewGenesisState creates a new genesis state for the evm module
func NewGenesisState(accounts []GenesisAccount, txLogs []TransactionLogs, wl AddressList, bl AddressList, config ChainConfig, params Params) *GenesisState {
	return &GenesisState{
		Accounts: accounts,
		TxsLogs:  txLogs,
		//ContractDeploymentWhitelist:wl,
		//ContractBlockedList:bl,
		ChainConfig: config,
		Params:      params,
	}
}

// DefaultGenesisState defines the default evm genesis state
func DefaultGenesisState() *GenesisState {
	return NewGenesisState(
		[]GenesisAccount{},
		[]TransactionLogs{},
		AddressList{},
		AddressList{},
		DefaultChainConfig(),
		DefaultParams(),
	)
}

// ValidateGenesis checks if parameters are within valid ranges
func ValidateGenesis(data *GenesisState) error {
	seenAccounts := make(map[string]bool)
	seenTxs := make(map[string]bool)
	for _, acc := range data.Accounts {
		if seenAccounts[acc.Address] {
			return fmt.Errorf("duplicated genesis account %s", acc.Address)
		}
		if err := acc.Validate(); err != nil {
			return fmt.Errorf("invalid genesis account %s: %w", acc.Address, err)
		}
		seenAccounts[acc.Address] = true
	}

	for _, tx := range data.TxsLogs {
		if seenTxs[tx.Hash] {
			return fmt.Errorf("duplicated logs from transaction %s", tx.Hash)
		}

		if err := tx.Validate(); err != nil {
			return fmt.Errorf("invalid logs from transaction %s: %w", tx.Hash, err)
		}

		seenTxs[tx.Hash] = true
	}

	if err := data.ChainConfig.Validate(); err != nil {
		return err
	}

	return data.Params.Validate()
}

// Validate performs a basic validation of a GenesisAccount fields.
func (ga GenesisAccount) Validate() error {
	if ga.Address == (ethcmn.Address{}.String()) {
		return fmt.Errorf("address cannot be the zero address %s", ga.Address)
	}
	if len(ga.Code) == 0 {
		return errors.New("code bytes cannot be empty")
	}

	return ga.Storage.Validate()
}

//
//func (ga GenesisAccount) MarshalJSON() ([]byte, error) {
//	formatState := &struct {
//		Address string  `json:"address"`
//		Code    string  `json:"code,omitempty"`
//		Storage Storage `json:"storage,omitempty"`
//	}{
//		Address: ga.Address,
//		Code:    ga.Code.String(),
//		Storage: ga.Storage,
//	}
//
//	if ga.Code == nil {
//		formatState.Code = ""
//	}
//	return json.Marshal(formatState)
//}
//
//func (ga *GenesisAccount) UnmarshalJSON(input []byte) error {
//	formatState := &struct {
//		Address string  `json:"address"`
//		Code    string  `json:"code,omitempty"`
//		Storage Storage `json:"storage,omitempty"`
//	}{}
//	if err := json.Unmarshal(input, &formatState); err != nil {
//		return err
//	}
//
//	ga.Address = formatState.Address
//	if formatState.Code == "" {
//		ga.Code = nil
//	} else {
//		ga.Code = hexutil.MustDecode(formatState.Code)
//	}
//	ga.Storage = formatState.Storage
//	return nil
//}
