package derive

import (
	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type UserDepositSource struct {
	L1BlockHash common.Hash
	LogIndex    uint64
}

// NOTE: Source domain `3` is deprecated and unused in the protocol. In an early version of the interop feature,
// it was used to signify the source domain of "deposit context window" transactions. This has since been removed
// in favor of access-list based checks in the `CrossL2Inbox` predeploy.
const (
	UserDepositSourceDomain      = 0
	L1InfoDepositSourceDomain    = 1
	UpgradeDepositSourceDomain   = 2
	InvalidatedBlockSourceDomain = 4
)

func (dep *UserDepositSource) SourceHash() common.Hash {
	var input [32 * 2]byte
	copy(input[:32], dep.L1BlockHash[:])
	binary.BigEndian.PutUint64(input[32*2-8:], dep.LogIndex)
	depositIDHash := crypto.Keccak256Hash(input[:])
	var domainInput [32 * 2]byte
	binary.BigEndian.PutUint64(domainInput[32-8:32], UserDepositSourceDomain)
	copy(domainInput[32:], depositIDHash[:])
	return crypto.Keccak256Hash(domainInput[:])
}

type L1InfoDepositSource struct {
	L1BlockHash common.Hash
	SeqNumber   uint64
}

func (dep *L1InfoDepositSource) SourceHash() common.Hash {
	var input [32 * 2]byte
	copy(input[:32], dep.L1BlockHash[:])
	binary.BigEndian.PutUint64(input[32*2-8:], dep.SeqNumber)
	depositIDHash := crypto.Keccak256Hash(input[:])

	var domainInput [32 * 2]byte
	binary.BigEndian.PutUint64(domainInput[32-8:32], L1InfoDepositSourceDomain)
	copy(domainInput[32:], depositIDHash[:])
	return crypto.Keccak256Hash(domainInput[:])
}

// UpgradeDepositSource implements the translation of upgrade-tx identity information to a deposit source-hash,
// which makes the deposit uniquely identifiable.
// System-upgrade transactions have their own domain for source-hashes,
// to not conflict with user-deposits or deposited L1 information.
// The intent identifies the upgrade-tx uniquely, in a human-readable way.
type UpgradeDepositSource struct {
	Intent string
}

func (dep *UpgradeDepositSource) SourceHash() common.Hash {
	intentHash := crypto.Keccak256Hash([]byte(dep.Intent))

	var domainInput [32 * 2]byte
	binary.BigEndian.PutUint64(domainInput[32-8:32], UpgradeDepositSourceDomain)
	copy(domainInput[32:], intentHash[:])
	return crypto.Keccak256Hash(domainInput[:])
}

// InvalidatedBlockSource identifies the invalidated optimistic-block system deposit-transaction.
type InvalidatedBlockSource struct {
	OutputRoot common.Hash
}

func (dep *InvalidatedBlockSource) SourceHash() common.Hash {
	var domainInput [32 * 2]byte
	binary.BigEndian.PutUint64(domainInput[32-8:32], InvalidatedBlockSourceDomain)
	copy(domainInput[32:], dep.OutputRoot[:])
	return crypto.Keccak256Hash(domainInput[:])
}
