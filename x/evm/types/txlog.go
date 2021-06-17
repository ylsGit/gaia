package types

import (
	"errors"
	"fmt"

	ethcmn "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// Validate performs a basic validation of a GenesisAccount fields.
func (tx TransactionLogs) Validate() error {
	if IsEmptyHash(tx.Hash) {
		return fmt.Errorf("hash cannot be the empty %s", tx.Hash)
	}

	for i, log := range tx.Logs {
		if err := ValidateLog(log); err != nil {
			return fmt.Errorf("invalid log %d: %w", i, err)
		}
		if log.TxHash != tx.Hash {
			return fmt.Errorf("log tx hash mismatch (%s ≠ %s)", log.TxHash, tx.Hash)
		}
	}
	return nil
}

// ValidateLog performs a basic validation of an ethereum Log fields.
func ValidateLog(log *Log) error {
	if log == nil {
		return errors.New("log cannot be nil")
	}
	if err := ValidateAddress(log.Address); err != nil {
		return fmt.Errorf("invalid log address %s", err)
	}
	if IsEmptyHash(log.BlockHash) {
		return fmt.Errorf("block hash cannot be the empty %s", log.BlockHash)
	}
	if log.BlockNumber == 0 {
		return errors.New("block number cannot be zero")
	}
	if IsEmptyHash(log.TxHash) {
		return fmt.Errorf("tx hash cannot be the empty %s", log.TxHash)
	}
	return nil
}

// EthLogs returns the Ethereum type Logs from the Transaction Logs.
func (tx TransactionLogs) EthLogs() []*ethtypes.Log {
	return LogsToEthereum(tx.Logs)
}

// Validate performs a basic validation of an ethereum Log fields.
func (log *Log) Validate() error {
	if err := ValidateAddress(log.Address); err != nil {
		return fmt.Errorf("invalid log address %w", err)
	}
	if IsEmptyHash(log.BlockHash) {
		return fmt.Errorf("block hash cannot be the empty %s", log.BlockHash)
	}
	if log.BlockNumber == 0 {
		return errors.New("block number cannot be zero")
	}
	if IsEmptyHash(log.TxHash) {
		return fmt.Errorf("tx hash cannot be the empty %s", log.TxHash)
	}
	return nil
}

// ToEthereum returns the Ethereum type Log from a Ethermint-proto compatible Log.
func (log *Log) ToEthereum() *ethtypes.Log {
	topics := make([]ethcmn.Hash, len(log.Topics))
	for i := range log.Topics {
		topics[i] = ethcmn.HexToHash(log.Topics[i])
	}

	return &ethtypes.Log{
		Address:     ethcmn.HexToAddress(log.Address),
		Topics:      topics,
		Data:        log.Data,
		BlockNumber: log.BlockNumber,
		TxHash:      ethcmn.HexToHash(log.TxHash),
		TxIndex:     uint(log.TxIndex),
		Index:       uint(log.Index),
		BlockHash:   ethcmn.HexToHash(log.BlockHash),
		Removed:     log.Removed,
	}
}

// LogsToEthereum casts the Ethermint Logs to a slice of Ethereum Logs.
func LogsToEthereum(logs []*Log) []*ethtypes.Log {
	ethLogs := make([]*ethtypes.Log, len(logs))
	for i := range logs {
		err := logs[i].Validate()
		if err != nil {
			continue
		}

		ethLogs[i] = logs[i].ToEthereum()
	}
	return ethLogs
}

// NewLogFromEth creates a new Log instance from a Ethereum type Log.
func NewLogFromEth(log *ethtypes.Log) *Log {
	topics := make([]string, len(log.Topics))
	for i := range log.Topics {
		topics[i] = log.Topics[i].String()
	}

	return &Log{
		Address:     log.Address.String(),
		Topics:      topics,
		Data:        log.Data,
		BlockNumber: log.BlockNumber,
		TxHash:      log.TxHash.String(),
		TxIndex:     uint64(log.TxIndex),
		BlockHash:   log.BlockHash.String(),
		Removed:     log.Removed,
	}
}

// NewLogsFromEth creates a new NewLogsFromEth instance using []*ethtypes.Log.
func NewLogsFromEth(ethlogs []*ethtypes.Log) []*Log { // nolint: interfacer
	logs := make([]*Log, len(ethlogs))
	for i := range ethlogs {
		logs[i] = NewLogFromEth(ethlogs[i])
	}

	return logs
}
