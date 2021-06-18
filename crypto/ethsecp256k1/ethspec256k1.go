package ethsecp256k1

import (
	"bytes"
	"crypto/ecdsa"
	fmt "fmt"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/tendermint/tendermint/crypto"

	// nolint: staticcheck // necessary for Bitcoin address format
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
)

var _ cryptotypes.PrivKey = &PrivKey{}

//var _ codec.AminoMarshaler = &PrivKey{}

const (
	PrivKeySize = 32
	KeyType     = "eth_secp256k1"
	PrivKeyName = "ethereum/PrivKeyEthSecp256k1"
	PubKeyName  = "ethereum/PubKeyEthSecp256k1"
)

// Bytes returns the byte representation of the Private Key.
func (privKey *PrivKey) Bytes() []byte {
	return privKey.Key
}

// PubKey performs the point-scalar multiplication from the privKey on the
// generator point to get the pubkey.
func (privKey *PrivKey) PubKey() cryptotypes.PubKey {
	ecdsaPKey := privKey.ToECDSA()
	pk := ethcrypto.CompressPubkey(&ecdsaPKey.PublicKey)
	return &PubKey{Key: pk}
}

// Equals - you probably don't need to use this.
// Runs in constant time based on length of the
func (privKey *PrivKey) Equals(other cryptotypes.LedgerPrivKey) bool {
	return privKey.Type() == other.Type() && bytes.Equal(privKey.Bytes(), other.Bytes())
}

func (privKey *PrivKey) Type() string {
	return KeyType
}

//
//// MarshalAmino overrides Amino binary marshalling.
//func (privKey PrivKey) MarshalAmino() ([]byte, error) {
//	return privKey.Key, nil
//}
//
//// UnmarshalAmino overrides Amino binary marshalling.
//func (privKey *PrivKey) UnmarshalAmino(bz []byte) error {
//	if len(bz) != PrivKeySize {
//		return fmt.Errorf("invalid privkey size")
//	}
//	privKey.Key = bz
//
//	return nil
//}
//
//// MarshalAminoJSON overrides Amino JSON marshalling.
//func (privKey PrivKey) MarshalAminoJSON() ([]byte, error) {
//	// When we marshal to Amino JSON, we don't marshal the "key" field itself,
//	// just its contents (i.e. the key bytes).
//	return privKey.MarshalAmino()
//}
//
//// UnmarshalAminoJSON overrides Amino JSON marshalling.
//func (privKey *PrivKey) UnmarshalAminoJSON(bz []byte) error {
//	return privKey.UnmarshalAmino(bz)
//}

// Sign creates a recoverable ECDSA signature on the secp256k1 curve over the
// Keccak256 hash of the provided message. The produced signature is 65 bytes
// where the last byte contains the recovery ID.
func (privkey PrivKey) Sign(msg []byte) ([]byte, error) {
	return ethcrypto.Sign(ethcrypto.Keccak256Hash(msg).Bytes(), privkey.ToECDSA())
}

// ToECDSA returns the ECDSA private key as a reference to ecdsa.PrivateKey type.
// The function will panic if the private key is invalid.
func (privkey *PrivKey) ToECDSA() *ecdsa.PrivateKey {
	key, err := ethcrypto.ToECDSA(privkey.Key)
	if err != nil {
		panic(err)
	}
	return key
}

// GenPrivKey generates a new ECDSA private key on curve secp256k1 private key.
// It uses OS randomness to generate the private key.
func GenPrivKey() (*PrivKey, error) {
	priv, err := ethcrypto.GenerateKey()
	if err != nil {
		return &PrivKey{}, err
	}

	return &PrivKey{ethcrypto.FromECDSA(priv)}, nil
}

//-------------------------------------

var _ cryptotypes.PubKey = &PubKey{}

//var _ codec.AminoMarshaler = &PubKey{}

// PubKeySize is comprised of 32 bytes for one field element
// (the x-coordinate), plus one byte for the parity of the y-coordinate.
const PubKeySize = 33

// Address returns a Bitcoin style addresses: RIPEMD160(SHA256(pubkey))
func (pubKey *PubKey) Address() crypto.Address {
	pubk, err := ethcrypto.DecompressPubkey(pubKey.Key)
	if err != nil {
		panic(err)
	}

	return crypto.Address(ethcrypto.PubkeyToAddress(*pubk).Bytes())
}

// Bytes returns the pubkey byte format.
func (pubKey *PubKey) Bytes() []byte {
	return pubKey.Key
}

func (pubKey *PubKey) String() string {
	return fmt.Sprintf("PubKeyETHSecp256k1{%X}", pubKey.Key)
}

func (pubKey *PubKey) Type() string {
	return KeyType
}

func (pubKey *PubKey) Equals(other cryptotypes.PubKey) bool {
	return pubKey.Type() == other.Type() && bytes.Equal(pubKey.Bytes(), other.Bytes())
}

//
//// MarshalAmino overrides Amino binary marshalling.
//func (pubKey PubKey) MarshalAmino() ([]byte, error) {
//	return pubKey.Key, nil
//}
//
//// UnmarshalAmino overrides Amino binary marshalling.
//func (pubKey *PubKey) UnmarshalAmino(bz []byte) error {
//	if len(bz) != PubKeySize {
//		return errors.Wrap(errors.ErrInvalidPubKey, "invalid pubkey size")
//	}
//	pubKey.Key = bz
//
//	return nil
//}
//
//// MarshalAminoJSON overrides Amino JSON marshalling.
//func (pubKey PubKey) MarshalAminoJSON() ([]byte, error) {
//	// When we marshal to Amino JSON, we don't marshal the "key" field itself,
//	// just its contents (i.e. the key bytes).
//	return pubKey.MarshalAmino()
//}
//
//// UnmarshalAminoJSON overrides Amino JSON marshalling.
//func (pubKey *PubKey) UnmarshalAminoJSON(bz []byte) error {
//	return pubKey.UnmarshalAmino(bz)
//}

// VerifyBytes verifies that the ECDSA public key created a given signature over
// the provided message. It will calculate the Keccak256 hash of the message
// prior to verification.
func (pubKey *PubKey) VerifySignature(msg []byte, sig []byte) bool {
	if len(sig) == 65 {
		// remove recovery ID if contained in the signature
		sig = sig[:len(sig)-1]
	}

	// the signature needs to be in [R || S] format when provided to VerifySignature
	return secp256k1.VerifySignature(pubKey.Key, ethcrypto.Keccak256Hash(msg).Bytes(), sig)
}
