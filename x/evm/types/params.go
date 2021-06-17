package types

import (
	"errors"
	"fmt"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/ethereum/go-ethereum/core/vm"
	yaml "gopkg.in/yaml.v2"
)

const (
	DefaultMaxGasLimitPerTx = 30000000
)

// Parameter keys
var (
	KeyEnableCreate                = []byte("EnableCreate")
	KeyEnableCall                  = []byte("EnableCall")
	KeyExtraEIPs                   = []byte("EnableExtraEIPs")
	KeyContractDeploymentWhitelist = []byte("EnableContractDeploymentWhitelist")
	KeyContractBlockedList         = []byte("EnableContractBlockedList")
	KeyMaxGasLimitPerTx            = []byte("MaxGasLimitPerTx")
	KeyEvmDenom                    = []byte("EvmDenom")
)

// ParamKeyTable returns the parameter key table.
func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

// NewParams creates a new Params instance
func NewParams(evmDenom string, enableCreate, enableCall, enableContractDeploymentWhitelist, enableContractBlockedList bool, maxGasLimitPerTx uint64,
	extraEIPs ...int) Params {
	return Params{
		EnableCreate:                      enableCreate,
		EnableCall:                        enableCall,
		ExtraEIPs:                         extraEIPs,
		EnableContractDeploymentWhitelist: enableContractDeploymentWhitelist,
		EnableContractBlockedList:         enableContractBlockedList,
		MaxGasLimitPerTx:                  maxGasLimitPerTx,
		EvmDenom:                          evmDenom,
	}
}

// DefaultParams returns default evm parameters
func DefaultParams() Params {
	return Params{
		EnableCreate:                      true,
		EnableCall:                        true,
		ExtraEIPs:                         []int(nil), // TODO: define default values
		EnableContractDeploymentWhitelist: false,
		EnableContractBlockedList:         false,
		MaxGasLimitPerTx:                  DefaultMaxGasLimitPerTx,
		EvmDenom:                          sdk.DefaultBondDenom,
	}
}

// Validate performs basic validation on evm parameters.
func (p Params) Validate() error {
	return validateEIPs(p.ExtraEIPs)
}

// String implements the fmt.Stringer interface
func (p Params) String() string {
	out, _ := yaml.Marshal(p)
	return string(out)
}

// ParamSetPairs implements params.ParamSet
func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{
		paramtypes.NewParamSetPair(KeyEnableCreate, &p.EnableCreate, validateBool),
		paramtypes.NewParamSetPair(KeyEnableCall, &p.EnableCall, validateBool),
		paramtypes.NewParamSetPair(KeyExtraEIPs, &p.ExtraEIPs, validateEIPs),
		paramtypes.NewParamSetPair(KeyContractDeploymentWhitelist, &p.EnableContractDeploymentWhitelist, validateBool),
		paramtypes.NewParamSetPair(KeyContractBlockedList, &p.EnableContractBlockedList, validateBool),
		paramtypes.NewParamSetPair(KeyMaxGasLimitPerTx, &p.MaxGasLimitPerTx, validateUint64),
		paramtypes.NewParamSetPair(KeyEvmDenom, &p.EvmDenom, validateBondDenom),
	}
}

func validateBool(i interface{}) error {
	_, ok := i.(bool)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	return nil
}

func validateEIPs(i interface{}) error {
	eips, ok := i.([]int)
	if !ok {
		return fmt.Errorf("invalid EIP slice type: %T", i)
	}

	for _, eip := range eips {
		if !vm.ValidEip(eip) {
			return fmt.Errorf("EIP %d is not activateable", eip)
		}
	}

	return nil
}

func validateUint64(i interface{}) error {
	_, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	return nil
}

func validateBondDenom(i interface{}) error {
	v, ok := i.(string)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if strings.TrimSpace(v) == "" {
		return errors.New("bond denom cannot be blank")
	}

	if err := sdk.ValidateDenom(v); err != nil {
		return err
	}

	return nil
}
