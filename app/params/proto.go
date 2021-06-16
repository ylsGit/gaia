package params

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	evmtypes "github.com/cosmos/cosmos-sdk/x/evm/types"
)

// MakeEncodingConfig creates an EncodingConfig for an amino based test configuration.
func MakeEncodingConfig() EncodingConfig {
	amino := codec.NewLegacyAmino()
	interfaceRegistry := types.NewInterfaceRegistry()
	marshaler := codec.NewProtoCodec(interfaceRegistry)
	txCfg := NewTxConfig(marshaler)

	return EncodingConfig{
		InterfaceRegistry: interfaceRegistry,
		Marshaler:         marshaler,
		TxConfig:          txCfg,
		Amino:             amino,
	}
}

type txConfig struct {
	cdc codec.ProtoCodecMarshaler
	client.TxConfig
}

// NewTxConfig returns a new protobuf TxConfig using the provided ProtoCodec and sign modes. The
// first enabled sign mode will become the default sign mode.
func NewTxConfig(marshaler codec.ProtoCodecMarshaler) client.TxConfig {
	return &txConfig{
		marshaler,
		tx.NewTxConfig(marshaler, tx.DefaultSignModes),
	}
}

// TxEncoder overwrites sdk.TxEncoder to support MsgEthereumTx
func (g txConfig) TxEncoder() sdk.TxEncoder {
	return func(tx sdk.Tx) ([]byte, error) {
		ethtx, ok := tx.(*evmtypes.MsgEthereumTx)
		if ok {
			return g.cdc.MarshalBinaryBare(ethtx)
		}
		return g.TxConfig.TxEncoder()(tx)
	}
}

// TxDecoder overwrites sdk.TxDecoder to support MsgEthereumTx
func (g txConfig) TxDecoder() sdk.TxDecoder {
	return func(txBytes []byte) (sdk.Tx, error) {
		var ethtx evmtypes.MsgEthereumTx

		err := g.cdc.UnmarshalBinaryBare(txBytes, &ethtx)
		if err == nil {
			return &ethtx, nil
		}

		return g.TxConfig.TxDecoder()(txBytes)
	}
}
