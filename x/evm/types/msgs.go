package types

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"io"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	ethcmn "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// Evm message types and routes
const (
	TypeMsgEthereumTx = "ethereum"
)

var (
	_ sdk.Msg    = &MsgEthereumTx{}
	_ sdk.Tx     = &MsgEthereumTx{}
	_ ante.GasTx = &MsgEthereumTx{}

	big8 = big.NewInt(8)
)

// NewMsgEthereumTx returns a reference to a new Ethereum transaction message.
func NewMsgEthereumTx(
	nonce uint64, to *ethcmn.Address, amount *big.Int,
	gasLimit uint64, gasPrice *big.Int, payload []byte,
) *MsgEthereumTx {
	return newMsgEthereumTx(nonce, to, amount, gasLimit, gasPrice, payload)
}

func newMsgEthereumTx(
	nonce uint64, to *ethcmn.Address, amount *big.Int,
	gasLimit uint64, gasPrice *big.Int, payload []byte,
) *MsgEthereumTx {
	if len(payload) > 0 {
		payload = ethcmn.CopyBytes(payload)
	}

	var toHex string
	if to != nil {
		toHex = to.String()
	}

	txData := TxData{
		AccountNonce: nonce,
		Recipient:    toHex,
		Payload:      payload,
		GasLimit:     gasLimit,
		Amount:       []byte{},
		Price:        []byte{},
		V:            []byte{},
		R:            []byte{},
		S:            []byte{},
	}

	if amount != nil {
		txData.Amount = amount.Bytes()
	}
	if gasPrice != nil {
		txData.Price = gasPrice.Bytes()
	}

	return &MsgEthereumTx{Data: &txData}
}

// Route implements the sdk.Msg interface.
func (msg MsgEthereumTx) Route() string { return RouterKey }

// Type implements the sdk.Msg interface.
func (msg MsgEthereumTx) Type() string { return TypeMsgEthereumTx }

// GetSigners implements the sdk.Msg interface.
func (msg MsgEthereumTx) GetSigners() []sdk.AccAddress {
	sender := msg.GetFrom()
	if sender.Empty() {
		panic("must use 'VerifySig' with a chain ID to get the signer")
	}
	return []sdk.AccAddress{sender}
}

// GetSignBytes implements the sdk.Msg interface.
func (msg MsgEthereumTx) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(&msg)
	return sdk.MustSortJSON(bz)
}

// ValidateBasic implements the sdk.Msg interface.
func (msg MsgEthereumTx) ValidateBasic() error {
	gasPrice := new(big.Int).SetBytes(msg.Data.Price)
	if gasPrice.Sign() == 0 {
		return sdkerrors.Wrapf(ErrInvalidValue, "gas price cannot be 0")
	}

	if gasPrice.Sign() == -1 {
		return sdkerrors.Wrapf(ErrInvalidValue, "gas price cannot be negative %s", msg.Data.Price)
	}

	// Amount can be 0
	amount := new(big.Int).SetBytes(msg.Data.Amount)
	if amount.Sign() == -1 {
		return sdkerrors.Wrapf(ErrInvalidValue, "amount cannot be negative %s", msg.Data.Amount)
	}

	if msg.From != "" {
		if err := ValidateAddress(msg.From); err != nil {
			return sdkerrors.Wrap(err, "invalid from address")
		}
	}

	return nil
}

// From loads the ethereum sender address from the sigcache and returns an
// sdk.AccAddress from its bytes
func (msg *MsgEthereumTx) GetFrom() sdk.AccAddress {
	if len(msg.From) == 0 {
		return nil
	}

	return sdk.AccAddress(ethcmn.HexToAddress(msg.From).Bytes())
}

func NewEIP155Signer(chainId *big.Int) *EIP155Signer {
	if chainId == nil {
		chainId = new(big.Int)
	}
	return &EIP155Signer{
		chainId:    chainId.Bytes(),
		chainIdMul: new(big.Int).Mul(chainId, big.NewInt(2)).Bytes(),
	}
}

// HomesteadSignHash returns the hash to be signed by the sender.
// It does not uniquely identify the transaction.
func (msg MsgEthereumTx) HomesteadSignHash() ethcmn.Hash {
	var recipient []byte
	if msg.Data.Recipient != "" {
		recipient = ethcmn.HexToAddress(msg.Data.Recipient).Bytes()
	}
	return rlpHash([]interface{}{
		msg.Data.AccountNonce,
		new(big.Int).SetBytes(msg.Data.Price),
		msg.Data.GasLimit,
		recipient,
		new(big.Int).SetBytes(msg.Data.Amount),
		msg.Data.Payload,
	})
}

// Sign calculates a secp256k1 ECDSA signature and signs the transaction. It
// takes a private key and chainID to sign an Ethereum transaction according to
// EIP155 standard. It mutates the transaction as it populates the V, R, S
// fields of the Transaction's Signature.
func (msg *MsgEthereumTx) Sign(chainID *big.Int, priv *ecdsa.PrivateKey) error {
	txHash := msg.RLPSignBytes(chainID)

	sig, err := ethcrypto.Sign(txHash[:], priv)
	if err != nil {
		return err
	}

	if len(sig) != 65 {
		return fmt.Errorf("wrong size for signature: got %d, want 65", len(sig))
	}

	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:64])

	var v *big.Int

	if chainID.Sign() == 0 {
		v = new(big.Int).SetBytes([]byte{sig[64] + 27})
	} else {
		v = big.NewInt(int64(sig[64] + 35))
		chainIDMul := new(big.Int).Mul(chainID, big.NewInt(2))

		v.Add(v, chainIDMul)
	}

	msg.Data.V = v.Bytes()
	msg.Data.R = r.Bytes()
	msg.Data.S = s.Bytes()
	return nil
}

// VerifySig attempts to verify a Transaction's signature for a given chainID.
// A derived address is returned upon success or an error if recovery fails.
func (msg *MsgEthereumTx) VerifySig(chainID *big.Int) (ethcmn.Address, error) {
	var signer ethtypes.Signer
	if isProtectedV(new(big.Int).SetBytes(msg.Data.V)) {
		signer = ethtypes.NewEIP155Signer(chainID)
	} else {
		signer = ethtypes.HomesteadSigner{}
	}

	if msg.Signer != nil {
		sc := ethtypes.NewEIP155Signer(new(big.Int).SetBytes(msg.Signer.chainId))
		// If the signer used to derive from in a previous call is not the same as
		// used current, invalidate the cache.
		if sc.Equal(signer) {
			return ethcmn.HexToAddress(msg.From), nil
		}
	}

	V := new(big.Int)
	var sigHash ethcmn.Hash
	if isProtectedV(new(big.Int).SetBytes(msg.Data.V)) {
		// do not allow recovery for transactions with an unprotected chainID
		if chainID.Sign() == 0 {
			return ethcmn.Address{}, errors.New("chainID cannot be zero")
		}

		chainIDMul := new(big.Int).Mul(chainID, big.NewInt(2))
		V = new(big.Int).Sub(new(big.Int).SetBytes(msg.Data.V), chainIDMul)
		V.Sub(V, big8)

		sigHash = msg.RLPSignBytes(chainID)
	} else {
		V = new(big.Int).SetBytes(msg.Data.V)

		sigHash = msg.HomesteadSignHash()
	}

	sender, err := recoverEthSig(new(big.Int).SetBytes(msg.Data.R), new(big.Int).SetBytes(msg.Data.S), V, sigHash)
	if err != nil {
		return ethcmn.Address{}, err
	}

	msg.Signer = NewEIP155Signer(chainID)
	msg.From = sender.String()
	return sender, nil
}

// RLPSignBytes returns the RLP hash of an Ethereum transaction message with a
// given chainID used for signing.
func (msg MsgEthereumTx) RLPSignBytes(chainID *big.Int) ethcmn.Hash {
	var recipient []byte
	if msg.Data.Recipient != "" {
		recipient = ethcmn.HexToAddress(msg.Data.Recipient).Bytes()
	}
	return rlpHash([]interface{}{
		msg.Data.AccountNonce,
		new(big.Int).SetBytes(msg.Data.Price),
		msg.Data.GasLimit,
		recipient,
		new(big.Int).SetBytes(msg.Data.Amount),
		msg.Data.Payload,
		chainID, uint(0), uint(0),
	})
}

// EncodeRLP implements the rlp.Encoder interface.
func (msg *MsgEthereumTx) EncodeRLP(w io.Writer) error {
	if msg.Data.Recipient != "" && ethcmn.IsHexAddress(msg.Data.Recipient) {
		msg.Data.Recipient = string(ethcmn.HexToAddress(msg.Data.Recipient).Bytes())
	}
	return rlp.Encode(w, &msg.Data)
}

// DecodeRLP implements the rlp.Decoder interface.
func (msg *MsgEthereumTx) DecodeRLP(s *rlp.Stream) error {
	_, size, err := s.Kind()
	if err != nil {
		// return error if stream is too large
		return err
	}

	if err := s.Decode(&msg.Data); err != nil {
		return err
	}

	if msg.Data.Recipient != "" && !ethcmn.IsHexAddress(msg.Data.Recipient) {
		msg.Data.Recipient = ethcmn.BytesToAddress([]byte(msg.Data.Recipient)).String()
	}

	msg.Size_ = float64(ethcmn.StorageSize(rlp.ListSize(size)))
	return nil
}

// codes from go-ethereum/core/types/transaction.go:122
func isProtectedV(V *big.Int) bool {
	if V.BitLen() <= 8 {
		v := V.Uint64()
		return v != 27 && v != 28
	}
	// anything not 27 or 28 is considered protected
	return true
}

// To returns the recipient address of the transaction. It returns nil if the
// transaction is a contract creation.
func (msg MsgEthereumTx) To() *ethcmn.Address {
	if msg.Data.Recipient == "" {
		return nil
	}

	recipient := ethcmn.HexToAddress(msg.Data.Recipient)
	return &recipient
}

// GetMsgs returns a single MsgEthereumTx as an sdk.Msg.
func (msg *MsgEthereumTx) GetMsgs() []sdk.Msg {
	return []sdk.Msg{msg}
}

// ChainID returns which chain id this transaction was signed for (if at all)
func (msg *MsgEthereumTx) ChainID() *big.Int {
	return deriveChainID(new(big.Int).SetBytes(msg.Data.V))
}

// deriveChainID derives the chain id from the given v parameter
func deriveChainID(v *big.Int) *big.Int {
	if v.BitLen() <= 64 {
		v := v.Uint64()
		if v == 27 || v == 28 {
			return new(big.Int)
		}
		return new(big.Int).SetUint64((v - 35) / 2)
	}
	v = new(big.Int).Sub(v, big.NewInt(35))
	return v.Div(v, big.NewInt(2))
}

// GetGas implements the GasTx interface. It returns the GasLimit of the transaction.
func (msg MsgEthereumTx) GetGas() uint64 {
	return msg.Data.GasLimit
}

// Fee returns gasprice * gaslimit.
func (msg MsgEthereumTx) Fee() *big.Int {
	return new(big.Int).Mul(new(big.Int).SetBytes(msg.Data.Price), new(big.Int).SetUint64(msg.Data.GasLimit))
}

// Cost returns amount + gasprice * gaslimit.
func (msg MsgEthereumTx) Cost() *big.Int {
	total := msg.Fee()
	total.Add(total, new(big.Int).SetBytes(msg.Data.Amount))
	return total
}

// GetTimeoutHeight returns the transaction's timeout height (if set).
func (msg *MsgEthereumTx) GetTimeoutHeight() uint64 {
	return 0
}
