package system

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/descriptors"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSystemFromEnv(t *testing.T) {
	// Create a temporary devnet file
	tempDir := t.TempDir()
	devnetFile := filepath.Join(tempDir, "devnet.json")

	devnet := &descriptors.DevnetEnvironment{
		L1: &descriptors.Chain{
			ID: "1",
			Nodes: []descriptors.Node{{
				Services: map[string]*descriptors.Service{
					"el": {
						Name: "geth",
						Endpoints: descriptors.EndpointMap{
							"rpc": &descriptors.PortInfo{
								Host: "localhost",
								Port: 8545,
							},
						},
					},
				},
			}},
			Wallets: descriptors.WalletMap{
				"default": &descriptors.Wallet{
					Address:    common.HexToAddress("0x123"),
					PrivateKey: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
				},
			},
			Addresses: descriptors.AddressMap{
				"defaultl1": common.HexToAddress("0x123"),
			},
		},
		L2: []*descriptors.L2Chain{
			{
				Chain: &descriptors.Chain{
					ID: "2",
					Nodes: []descriptors.Node{{
						Services: map[string]*descriptors.Service{
							"el": {
								Name: "geth",
								Endpoints: descriptors.EndpointMap{
									"rpc": &descriptors.PortInfo{
										Host: "localhost",
										Port: 8546,
									},
								},
							},
						},
					}},
					Wallets: descriptors.WalletMap{
						"default": &descriptors.Wallet{
							Address:    common.HexToAddress("0x123"),
							PrivateKey: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
						},
					},
					Addresses: descriptors.AddressMap{
						"defaultl2": common.HexToAddress("0x456"),
					},
				},
				L1Addresses: descriptors.AddressMap{
					"defaultl1": common.HexToAddress("0x123"),
				},
				L1Wallets: descriptors.WalletMap{
					"default": &descriptors.Wallet{
						Address:    common.HexToAddress("0x123"),
						PrivateKey: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
					},
				},
			},
		},
		Features: []string{},
	}

	data, err := json.Marshal(devnet)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(devnetFile, data, 0644))

	sys, err := NewSystemFromURL(devnetFile)
	assert.NoError(t, err)
	assert.NotNil(t, sys)
}

func TestSystemFromDevnet(t *testing.T) {
	testNode := descriptors.Node{
		Services: map[string]*descriptors.Service{
			"el": {
				Name: "geth",
				Endpoints: descriptors.EndpointMap{
					"rpc": &descriptors.PortInfo{
						Host: "localhost",
						Port: 8545,
					},
				},
			},
		},
	}

	testWallet := &descriptors.Wallet{
		Address:    common.HexToAddress("0x123"),
		PrivateKey: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
	}

	tests := []struct {
		name      string
		devnet    *descriptors.DevnetEnvironment
		wantErr   bool
		isInterop bool
	}{
		{
			name: "basic system",
			devnet: &descriptors.DevnetEnvironment{
				L1: &descriptors.Chain{
					ID:    "1",
					Nodes: []descriptors.Node{testNode},
					Wallets: descriptors.WalletMap{
						"default": testWallet,
					},
					Addresses: descriptors.AddressMap{
						"defaultl1": common.HexToAddress("0x123"),
					},
				},
				L2: []*descriptors.L2Chain{
					{
						Chain: &descriptors.Chain{
							ID:    "2",
							Nodes: []descriptors.Node{testNode},
							Wallets: descriptors.WalletMap{
								"default": testWallet,
							},
						},
						L1Addresses: descriptors.AddressMap{
							"defaultl1": common.HexToAddress("0x123"),
						},
						L1Wallets: descriptors.WalletMap{
							"default": testWallet,
						},
					},
				},
			},
			wantErr:   false,
			isInterop: false,
		},
		{
			name: "interop system",
			devnet: &descriptors.DevnetEnvironment{
				L1: &descriptors.Chain{
					ID:    "1",
					Nodes: []descriptors.Node{testNode},
					Wallets: descriptors.WalletMap{
						"default": testWallet,
					},
					Addresses: descriptors.AddressMap{
						"defaultl1": common.HexToAddress("0x123"),
					},
				},
				L2: []*descriptors.L2Chain{
					{
						Chain: &descriptors.Chain{
							ID:    "2",
							Nodes: []descriptors.Node{testNode},
							Wallets: descriptors.WalletMap{
								"default": testWallet,
							},
							Services: descriptors.RedundantServiceMap{
								"supervisor": []*descriptors.Service{
									&descriptors.Service{
										Name: "supervisor",
										Endpoints: descriptors.EndpointMap{
											"rpc": &descriptors.PortInfo{
												Host: "localhost",
												Port: 8545,
											},
										},
									},
								},
							},
						},
						L1Addresses: descriptors.AddressMap{
							"defaultl1": common.HexToAddress("0x123"),
						},
						L1Wallets: descriptors.WalletMap{
							"default": testWallet,
						},
					}},
				Features: []string{"interop"},
			},
			wantErr:   false,
			isInterop: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sys, err := systemFromDevnet(tt.devnet)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, sys)

			_, isInterop := sys.(InteropSystem)
			assert.Equal(t, tt.isInterop, isInterop)
		})
	}
}

func TestWallet(t *testing.T) {
	chain := newChain("1", WalletMap{}, nil, AddressMap{}, []Node{})
	tests := []struct {
		name        string
		privateKey  string
		address     types.Address
		wantAddr    types.Address
		wantPrivKey types.Key
	}{
		{
			name:       "valid wallet",
			privateKey: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			address:    common.HexToAddress("0x123"),
			wantAddr:   common.HexToAddress("0x123"),
		},
		{
			name:       "empty wallet",
			privateKey: "",
			address:    common.HexToAddress("0x123"),
			wantAddr:   common.HexToAddress("0x123"),
		},
		{
			name:       "only address",
			privateKey: "",
			address:    common.HexToAddress("0x456"),
			wantAddr:   common.HexToAddress("0x456"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, err := NewWallet(tt.privateKey, tt.address, chain)
			assert.Nil(t, err)

			assert.Equal(t, tt.wantAddr, w.Address())
		})
	}
}

func TestChainUser(t *testing.T) {
	chain := newChain("1", WalletMap{}, nil, AddressMap{}, []Node{})

	testWallet, err := NewWallet("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", common.HexToAddress("0x123"), chain)
	assert.Nil(t, err)

	chain.wallets = WalletMap{
		"l2Faucet": testWallet,
	}

	wallets := chain.Wallets()
	require.NoError(t, err)

	for _, w := range wallets {
		if w.Address() == testWallet.Address() {
			assert.Equal(t, testWallet.Address(), w.Address())
			assert.Equal(t, testWallet.PrivateKey(), w.PrivateKey())
			return
		}
	}
	assert.Fail(t, "wallet not found")
}
