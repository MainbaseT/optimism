package sysext

import (
	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/devnet-sdk/descriptors"
	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/stack"
)

const (
	ProtocolVersionsAddressName = "ProtocolVersionsProxy"
	SuperchainConfigAddressName = "SuperchainConfigProxy"

	SystemConfigAddressName   = "SystemConfigProxy"
	DisputeGameFactoryName    = "DisputeGameFactoryProxy"
	L1StandardBridgeProxyName = "L1StandardBridgeProxy"
)

type l1AddressBook struct {
	protocolVersions common.Address
	superchainConfig common.Address
}

func newL1AddressBook(t devtest.T, addresses descriptors.AddressMap) *l1AddressBook {
	// TODO(#15817) op-devstack: sysext: fix address book
	return &l1AddressBook{}
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
	l1StandardBridge   common.Address
}

func newL2AddressBook(t devtest.T, l1Addresses descriptors.AddressMap) *l2AddressBook {
	// TODO(#15817) op-devstack: sysext: fix address book
	return &l2AddressBook{
		systemConfig:       l1Addresses[SystemConfigAddressName],
		disputeGameFactory: l1Addresses[DisputeGameFactoryName],
		l1StandardBridge:   l1Addresses[L1StandardBridgeProxyName],
	}
}

func (a *l2AddressBook) SystemConfigProxyAddr() common.Address {
	return a.systemConfig
}

func (a *l2AddressBook) DisputeGameFactoryProxyAddr() common.Address {
	return a.disputeGameFactory
}

func (a *l2AddressBook) L1StandardBridgeProxyAddr() common.Address {
	return a.l1StandardBridge
}

var _ stack.L2Deployment = (*l2AddressBook)(nil)
