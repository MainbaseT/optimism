package syskt

import (
	"github.com/ethereum-optimism/optimism/devnet-sdk/descriptors"
	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/stack"
	"github.com/ethereum/go-ethereum/common"
)

const (
	ProtocolVersionsAddressName = "protocolVersionsProxy"
	SuperchainConfigAddressName = "superchainConfigProxy"

	SystemConfigAddressName = "systemConfigProxy"
	DisputeGameFactoryName  = "disputeGameFactoryProxy"
)

type l1AddressBook struct {
	protocolVersions common.Address
	superchainConfig common.Address
}

func newL1AddressBook(setup *stack.Setup, addresses descriptors.AddressMap) *l1AddressBook {
	protocolVersions, ok := addresses[ProtocolVersionsAddressName]
	setup.Require.True(ok)
	superchainConfig, ok := addresses[SuperchainConfigAddressName]
	setup.Require.True(ok)

	book := &l1AddressBook{
		protocolVersions: protocolVersions,
		superchainConfig: superchainConfig,
	}

	return book
}

func (a *l1AddressBook) ProtocolVersionsAddr() common.Address {
	return a.protocolVersions
}

func (a *l1AddressBook) SuperchainConfigAddr() common.Address {
	return a.superchainConfig
}

var _ stack.SuperchainDeployment = (*l1AddressBook)(nil)

type l2AddressBook struct {
	systemConfig       common.Address
	disputeGameFactory common.Address
}

func newL2AddressBook(setup *stack.Setup, l1Addresses descriptors.AddressMap) *l2AddressBook {
	systemConfig, ok := l1Addresses[SystemConfigAddressName]
	setup.Require.True(ok)
	disputeGameFactory, ok := l1Addresses[DisputeGameFactoryName]
	setup.Require.True(ok)

	return &l2AddressBook{
		systemConfig:       systemConfig,
		disputeGameFactory: disputeGameFactory,
	}
}

func (a *l2AddressBook) SystemConfigProxyAddr() common.Address {
	return a.systemConfig
}

func (a *l2AddressBook) DisputeGameFactoryProxyAddr() common.Address {
	return a.disputeGameFactory
}

var _ stack.L2Deployment = (*l2AddressBook)(nil)
