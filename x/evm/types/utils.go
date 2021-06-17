package types

import (
	"bytes"
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/gaia/v4/crypto/ethsecp256k1"
	ethcmn "github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/gogo/protobuf/codec"
	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"
	"golang.org/x/crypto/sha3"
)

// GenerateEthAddress generates an Ethereum address.
func GenerateEthAddress() ethcmn.Address {
	priv, err := ethsecp256k1.GenPrivKey()
	if err != nil {
		panic(err)
	}

	return ethcrypto.PubkeyToAddress(priv.ToECDSA().PublicKey)
}

// ValidateSigner attempts to validate a signer for a given slice of bytes over
// which a signature and signer is given. An error is returned if address
// derived from the signature and bytes signed does not match the given signer.
func ValidateSigner(signBytes, sig []byte, signer ethcmn.Address) error {
	pk, err := ethcrypto.SigToPub(signBytes, sig)

	if err != nil {
		return errors.Wrap(err, "failed to derive public key from signature")
	} else if ethcrypto.PubkeyToAddress(*pk) != signer {
		return fmt.Errorf("invalid signature for signer: %s", signer)
	}

	return nil
}

func rlpHash(x interface{}) (hash ethcmn.Hash) {
	hasher := sha3.NewLegacyKeccak256()
	_ = rlp.Encode(hasher, x)
	_ = hasher.Sum(hash[:0])

	return hash
}

//// ResultData represents the data returned in an sdk.Result
//type ResultData struct {
//	ContractAddress ethcmn.Address  `json:"contract_address"`
//	Bloom           ethtypes.Bloom  `json:"bloom"`
//	Logs            []*ethtypes.Log `json:"logs"`
//	Ret             []byte          `json:"ret"`
//	TxHash          ethcmn.Hash     `json:"tx_hash"`
//}
//
//// String implements fmt.Stringer interface.
//func (rd ResultData) String() string {
//	var logsStr string
//	logsLen := len(rd.Logs)
//	for i := 0; i < logsLen; i++ {
//		logsStr = fmt.Sprintf("%s\t\t%v\n ", logsStr, *rd.Logs[i])
//	}
//
//	return strings.TrimSpace(fmt.Sprintf(`ResultData:
//	ContractAddress: %s
//	Bloom: %s
//	Ret: %v
//	TxHash: %s
//	Logs:
//%s`, rd.ContractAddress.String(), rd.Bloom.Big().String(), rd.Ret, rd.TxHash.String(), logsStr))
//}

// EncodeTxResponse takes all of the necessary data from the EVM execution
// and returns the data as a byte slice encoded with protobuf.
func EncodeTxResponse(res *MsgEthereumTxResponse) ([]byte, error) {
	return proto.Marshal(res)
}

// DecodeTxResponse decodes an protobuf-encoded byte slice into TxResponse
func DecodeTxResponse(in []byte) (*MsgEthereumTxResponse, error) {
	var txMsgData sdk.TxMsgData
	if err := proto.Unmarshal(in, &txMsgData); err != nil {
		return nil, err
	}

	dataList := txMsgData.GetData()
	if len(dataList) == 0 {
		return &MsgEthereumTxResponse{}, nil
	}

	var res MsgEthereumTxResponse

	err := proto.Unmarshal(dataList[0].GetData(), &res)
	if err != nil {
		err = errors.Wrap(err, "proto.Unmarshal failed")
		return nil, err
	}

	return &res, nil
}

// ----------------------------------------------------------------------------
// Auxiliary

// TxDecoder returns an sdk.TxDecoder that can decode both auth.StdTx and
// MsgEthereumTx transactions.
func TxDecoder(cdc *codec.Codec) sdk.TxDecoder {
	return func(txBytes []byte) (sdk.Tx, error) {
		var tx sdk.Tx
		//
		//if len(txBytes) == 0 {
		//	return nil, sdkerrors.Wrap(sdkerrors.ErrTxDecode, "tx bytes are empty")
		//}
		//
		//// sdk.Tx is an interface. The concrete message types
		//// are registered by MakeTxCodec
		//// TODO: switch to UnmarshalBinaryBare on SDK v0.40.0
		//err := cdc.UnmarshalBinaryLengthPrefixed(txBytes, &tx)
		//if err != nil {
		//	return nil, sdkerrors.Wrap(sdkerrors.ErrTxDecode, err.Error())
		//}

		return tx, nil
	}
}

// recoverEthSig recovers a signature according to the Ethereum specification and
// returns the sender or an error.
//
// Ref: Ethereum Yellow Paper (BYZANTIUM VERSION 69351d5) Appendix F
// nolint: gocritic
func recoverEthSig(R, S, Vb *big.Int, sigHash ethcmn.Hash) (ethcmn.Address, error) {
	if Vb.BitLen() > 8 {
		return ethcmn.Address{}, errors.New("invalid signature")
	}

	V := byte(Vb.Uint64() - 27)
	if !ethcrypto.ValidateSignatureValues(V, R, S, true) {
		return ethcmn.Address{}, errors.New("invalid signature")
	}

	// encode the signature in uncompressed format
	r, s := R.Bytes(), S.Bytes()
	sig := make([]byte, 65)

	copy(sig[32-len(r):32], r)
	copy(sig[64-len(s):64], s)
	sig[64] = V

	// recover the public key from the signature
	pub, err := ethcrypto.Ecrecover(sigHash[:], sig)
	if err != nil {
		return ethcmn.Address{}, err
	}

	if len(pub) == 0 || pub[0] != 4 {
		return ethcmn.Address{}, errors.New("invalid public key")
	}

	var addr ethcmn.Address
	copy(addr[:], ethcrypto.Keccak256(pub[1:])[12:])

	return addr, nil
}

// IsEmptyHash returns true if the hash corresponds to an empty ethereum hex hash.
func IsEmptyHash(hash string) bool {
	return bytes.Equal(ethcmn.HexToHash(hash).Bytes(), ethcmn.Hash{}.Bytes())
}

// IsZeroAddress returns true if the address corresponds to an empty ethereum hex address.
func IsZeroAddress(address string) bool {
	return bytes.Equal(ethcmn.HexToAddress(address).Bytes(), ethcmn.Address{}.Bytes())
}

// ValidateAddress returns an error if the provided string is either not a hex formatted string address
// the it matches the zero address 0x00000000000000000000.
func ValidateAddress(address string) error {
	if !ethcmn.IsHexAddress(address) {
		return sdkerrors.Wrapf(
			sdkerrors.ErrInvalidAddress, "address '%s' is not a valid ethereum hex address",
			address,
		)
	}

	if IsZeroAddress(address) {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, "provided address cannot be the zero address")
	}

	return nil
}
