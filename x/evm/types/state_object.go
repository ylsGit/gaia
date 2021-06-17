package types

import (
	"bytes"
	"fmt"
	"io"
	"math/big"

	ethcmn "github.com/ethereum/go-ethereum/common"
	ethstate "github.com/ethereum/go-ethereum/core/state"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

var (
	_ StateObject = (*stateObject)(nil)

	emptyCodeHash = ethcrypto.Keccak256(nil)
)

// StateObject interface for interacting with state object
type StateObject interface {
	GetCommittedState(db ethstate.Database, key ethcmn.Hash) ethcmn.Hash
	GetState(db ethstate.Database, key ethcmn.Hash) ethcmn.Hash
	SetState(db ethstate.Database, key, value ethcmn.Hash)

	Code(db ethstate.Database) []byte
	SetCode(codeHash ethcmn.Hash, code []byte)
	CodeHash() []byte

	AddBalance(amount *big.Int)
	SubBalance(amount *big.Int)
	SetBalance(amount *big.Int)

	Balance() *big.Int
	ReturnGas(gas *big.Int)
	Address() ethcmn.Address

	SetNonce(nonce uint64)
	Nonce() uint64
}

// stateObject represents an Ethereum account which is being modified.
//
// The usage pattern is as follows:
// First you need to obtain a state object.
// Account values can be accessed and modified through the object.
// Finally, call CommitTrie to write the modified storage trie into a database.
type stateObject struct {
	code Code // contract bytecode, which gets set when code is loaded
	// State objects are used by the consensus core and VM which are
	// unable to deal with database-level errors. Any error that occurs
	// during a database read is memoized here and will eventually be returned
	// by StateDB.Commit.
	originStorage Storage // Storage cache of original entries to dedup rewrites
	dirtyStorage  Storage // Storage entries that need to be flushed to disk

	// DB error
	dbErr   error
	stateDB *CommitStateDB
	account *EthAccount
	// balance represents the amount of the EVM denom token that an account holds
	balance sdk.Dec

	keyToOriginStorageIndex map[ethcmn.Hash]int
	keyToDirtyStorageIndex  map[ethcmn.Hash]int

	address ethcmn.Address

	// cache flags
	//
	// When an object is marked suicided it will be delete from the trie during
	// the "update" phase of the state transition.
	dirtyCode bool // true if the code was updated
	suicided  bool
	deleted   bool
}

func newStateObject(db *CommitStateDB, accProto authtypes.AccountI, balance sdk.Int) *stateObject {
	// func newStateObject(db *CommitStateDB, accProto authexported.Account, balance sdk.Int) *stateObject {
	ethermintAccount, ok := accProto.(*EthAccount)
	if !ok {
		panic(fmt.Sprintf("invalid account type for state object: %T", accProto))
	}

	// set empty code hash
	if ethermintAccount.CodeHash == nil {
		ethermintAccount.CodeHash = emptyCodeHash
	}

	amt := sdk.NewDecFromBigInt(balance.BigInt()) // int2dec * 10^18

	return &stateObject{
		stateDB:                 db,
		account:                 ethermintAccount,
		balance:                 amt,
		address:                 ethermintAccount.EthAddress(),
		originStorage:           Storage{},
		dirtyStorage:            Storage{},
		keyToOriginStorageIndex: make(map[ethcmn.Hash]int),
		keyToDirtyStorageIndex:  make(map[ethcmn.Hash]int),
	}
}

// ----------------------------------------------------------------------------
// Setters
// ----------------------------------------------------------------------------

// SetState updates a value in account storage. Note, the key will be prefixed
// with the address of the state object.
func (so *stateObject) SetState(db ethstate.Database, key, value ethcmn.Hash) {
	// if the new value is the same as old, don't set
	prev := so.GetState(db, key)
	if prev == value {
		return
	}

	prefixKey := so.GetStorageByAddressKey(key.Bytes())

	// since the new value is different, update and journal the change
	so.stateDB.journal.append(storageChange{
		account:   &so.address,
		key:       prefixKey,
		prevValue: prev,
	})

	so.setState(prefixKey, value)
}

// setState sets a state with a prefixed key and value to the dirty storage.
func (so *stateObject) setState(key, value ethcmn.Hash) {
	idx, ok := so.keyToDirtyStorageIndex[key]
	if ok {
		so.dirtyStorage[idx].Value = value.String()
		return
	}

	// create new entry
	so.dirtyStorage = append(so.dirtyStorage, NewState(key, value))
	idx = len(so.dirtyStorage) - 1
	so.keyToDirtyStorageIndex[key] = idx
}

// SetCode sets the state object's code.
func (so *stateObject) SetCode(codeHash ethcmn.Hash, code []byte) {
	prevCode := so.Code(nil)

	so.stateDB.journal.append(codeChange{
		account:  &so.address,
		prevHash: so.CodeHash(),
		prevCode: prevCode,
	})

	so.setCode(codeHash, code)
}

func (so *stateObject) setCode(codeHash ethcmn.Hash, code []byte) {
	so.code = code
	so.account.CodeHash = codeHash.Bytes()
	so.dirtyCode = true
}

// AddBalance adds an amount to a state object's balance. It is used to add
// funds to the destination account of a transfer.
func (so *stateObject) AddBalance(amount *big.Int) {
	amt := sdk.NewDecFromBigIntWithPrec(amount, sdk.Precision) // int2dec
	// EIP158: We must check emptiness for the objects such that the account
	// clearing (0,0,0 objects) can take effect.

	// NOTE: this will panic if amount is nil
	if amt.IsZero() {
		if so.empty() {
			so.touch()
		}
		return
	}

	so.SetBalance(so.balance.Add(amt).BigInt())
}

// SubBalance removes an amount from the stateObject's balance. It is used to
// remove funds from the origin account of a transfer.
func (so *stateObject) SubBalance(amount *big.Int) {
	amt := sdk.NewDecFromBigIntWithPrec(amount, sdk.Precision) // int2dec
	if amt.IsZero() {
		return
	}

	so.SetBalance(so.balance.Sub(amt).BigInt())
}

// SetBalance sets the state object's balance.
func (so *stateObject) SetBalance(amount *big.Int) {
	amt := sdk.NewDecFromBigIntWithPrec(amount, sdk.Precision) // int2dec
	so.stateDB.journal.append(balanceChange{
		account: &so.address,
		prev:    so.balance, // int2dec
	})

	so.setBalance(amt)
}

func (so *stateObject) setBalance(amount sdk.Dec) {
	so.balance = amount
}

// SetNonce sets the state object's nonce (i.e sequence number of the account).
func (so *stateObject) SetNonce(nonce uint64) {
	so.stateDB.journal.append(nonceChange{
		account: &so.address,
		prev:    so.account.Sequence,
	})

	so.setNonce(nonce)
}

func (so *stateObject) setNonce(nonce uint64) {
	if so.account == nil {
		panic("state object account is empty")
	}
	so.account.Sequence = nonce
}

// setError remembers the first non-nil error it is called with.
func (so *stateObject) setError(err error) {
	if so.dbErr == nil {
		so.dbErr = err
	}
}

func (so *stateObject) markSuicided() {
	so.suicided = true
}

// commitState commits all dirty storage to a KVStore and resets
// the dirty storage slice to the empty state.
func (so *stateObject) commitState() {
	ctx := so.stateDB.ctx
	store := prefix.NewStore(ctx.KVStore(so.stateDB.storeKey), AddressStoragePrefix(so.Address()))

	for _, state := range so.dirtyStorage {
		// NOTE: key is already prefixed from GetStorageByAddressKey

		key := ethcmn.HexToHash(state.Key)
		value := ethcmn.HexToHash(state.Value)
		// delete empty values from the store
		if IsEmptyHash(state.Value) {
			store.Delete(key.Bytes())
		}

		delete(so.keyToDirtyStorageIndex, key)

		// skip no-op changes, persist actual changes
		idx, ok := so.keyToOriginStorageIndex[key]
		if !ok {
			continue
		}

		if IsEmptyHash(state.Value) {
			delete(so.keyToOriginStorageIndex, key)
			continue
		}

		if state.Value == so.originStorage[idx].Value {
			continue
		}

		so.originStorage[idx].Value = state.Value
		store.Set(key.Bytes(), value.Bytes())
	}
	// clean storage as all entries are dirty
	so.dirtyStorage = Storage{}
}

// commitCode persists the state object's code to the KVStore.
func (so *stateObject) commitCode() {
	ctx := so.stateDB.ctx
	store := prefix.NewStore(ctx.KVStore(so.stateDB.storeKey), KeyPrefixCode)
	store.Set(so.CodeHash(), so.code)
}

// ----------------------------------------------------------------------------
// Getters
// ----------------------------------------------------------------------------

// Address returns the address of the state object.
func (so stateObject) Address() ethcmn.Address {
	return so.address
}

// Balance returns the state object's current balance.
func (so *stateObject) Balance() *big.Int {
	balance := so.balance.BigInt()
	if balance == nil {
		return zeroBalance
	}
	return balance
}

// CodeHash returns the state object's code hash.
func (so *stateObject) CodeHash() []byte {
	if so.account == nil || len(so.account.CodeHash) == 0 {
		return emptyCodeHash
	}
	return so.account.CodeHash
}

// Nonce returns the state object's current nonce (sequence number).
func (so *stateObject) Nonce() uint64 {
	if so.account == nil {
		return 0
	}
	return so.account.Sequence
}

// Code returns the contract code associated with this object, if any.
func (so *stateObject) Code(_ ethstate.Database) []byte {
	if len(so.code) > 0 {
		return so.code
	}

	if bytes.Equal(so.CodeHash(), emptyCodeHash) {
		return nil
	}

	ctx := so.stateDB.ctx
	store := prefix.NewStore(ctx.KVStore(so.stateDB.storeKey), KeyPrefixCode)
	code := store.Get(so.CodeHash())

	if len(code) == 0 {
		so.setError(fmt.Errorf("failed to get code hash %x for address %s", so.CodeHash(), so.Address().String()))
	}

	return code
}

// GetState retrieves a value from the account storage trie. Note, the key will
// be prefixed with the address of the state object.
func (so *stateObject) GetState(db ethstate.Database, key ethcmn.Hash) ethcmn.Hash {
	prefixKey := so.GetStorageByAddressKey(key.Bytes())

	// if we have a dirty value for this state entry, return it
	idx, dirty := so.keyToDirtyStorageIndex[prefixKey]
	if dirty {
		return ethcmn.HexToHash(so.dirtyStorage[idx].Value)
	}

	// otherwise return the entry's original value
	value := so.GetCommittedState(db, key)
	return value
}

// GetCommittedState retrieves a value from the committed account storage trie.
//
// NOTE: the key will be prefixed with the address of the state object.
func (so *stateObject) GetCommittedState(_ ethstate.Database, key ethcmn.Hash) ethcmn.Hash {
	prefixKey := so.GetStorageByAddressKey(key.Bytes())

	// if we have the original value cached, return that
	idx, cached := so.keyToOriginStorageIndex[prefixKey]
	if cached {
		return ethcmn.HexToHash(so.originStorage[idx].Value)
	}

	// otherwise load the value from the KVStore
	state := NewState(prefixKey, ethcmn.Hash{})
	value := ethcmn.Hash{}

	ctx := so.stateDB.ctx
	store := prefix.NewStore(ctx.KVStore(so.stateDB.storeKey), AddressStoragePrefix(so.Address()))
	rawValue := store.Get(prefixKey.Bytes())

	if len(rawValue) > 0 {
		value.SetBytes(rawValue)
		state.Value = value.String()
	}

	so.originStorage = append(so.originStorage, state)
	so.keyToOriginStorageIndex[prefixKey] = len(so.originStorage) - 1
	return value
}

// ----------------------------------------------------------------------------
// Auxiliary
// ----------------------------------------------------------------------------

// ReturnGas returns the gas back to the origin. Used by the Virtual machine or
// Closures. It performs a no-op.
func (so *stateObject) ReturnGas(gas *big.Int) {}

func (so *stateObject) deepCopy(db *CommitStateDB) *stateObject {
	newAccount := ProtoAccount().(*EthAccount)
	jsonAccount, err := so.account.MarshalJSON()
	if err != nil {
		return nil
	}
	err = newAccount.UnmarshalJSON(jsonAccount)
	if err != nil {
		return nil
	}
	newStateObj := newStateObject(db, newAccount, so.balance.RoundInt())

	newStateObj.code = make(Code, len(so.code))
	copy(newStateObj.code, so.code)
	newStateObj.dirtyStorage = so.dirtyStorage.Copy()
	newStateObj.originStorage = so.originStorage.Copy()
	newStateObj.suicided = so.suicided
	newStateObj.dirtyCode = so.dirtyCode
	newStateObj.deleted = so.deleted

	return newStateObj
}

// empty returns whether the account is considered empty.
func (so *stateObject) empty() bool {
	return so.account == nil ||
		(so.account != nil &&
			so.account.Sequence == 0 &&
			(so.balance.BigInt() == nil || so.balance.IsZero()) &&
			bytes.Equal(so.account.CodeHash, emptyCodeHash))
}

// EncodeRLP implements rlp.Encoder.
func (so *stateObject) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, so.account)
}

func (so *stateObject) touch() {
	so.stateDB.journal.append(touchChange{
		account: &so.address,
	})

	if so.address == ripemd {
		// Explicitly put it in the dirty-cache, which is otherwise generated from
		// flattened journals.
		so.stateDB.journal.dirty(so.address)
	}
}

// GetStorageByAddressKey returns a hash of the composite key for a state
// object's storage prefixed with it's address.
func (so stateObject) GetStorageByAddressKey(key []byte) ethcmn.Hash {
	prefix := so.Address().Bytes()
	compositeKey := make([]byte, len(prefix)+len(key))

	copy(compositeKey, prefix)
	copy(compositeKey[len(prefix):], key)

	return ethcrypto.Keccak256Hash(compositeKey)
}

// stateEntry represents a single key value pair from the StateDB's stateObject mappindg.
// This is to prevent non determinism at genesis initialization or export.
type stateEntry struct {
	// address key of the state object
	address     ethcmn.Address
	stateObject *stateObject
}
