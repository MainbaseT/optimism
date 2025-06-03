package validations

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

const (
	VersionV180 = "v1.8.0"
	VersionV200 = "v2.0.0"
	VersionV300 = "v3.0.0"
	VersionV400 = "v4.0.0"
)

var addresses = map[uint64]map[string]common.Address{
	1: {
		// Bootstrapped on 03/07/2025 using OP Deployer.
		VersionV180: common.HexToAddress("0x37fb5b21750d0e08a992350574bd1c24f4bcedf9"),
		// Bootstrapped on 03/07/2025 using OP Deployer.
		VersionV200: common.HexToAddress("0x12a9e38628e5a5b24d18b1956ed68a24fe4e3dc0"),
		// Bootstrapped on 04/16/2025 using OP Deployer.
		VersionV300: common.HexToAddress("0xf989Df70FB46c581ba6157Ab335c0833bA60e1f0"),
		// Bootstrapped on 06/03/2025 using OP Deployer.
		VersionV400: common.HexToAddress("0x3246b056A3C9B14AA25b75ee133DC3747476DBc5"),
	},
	11155111: {
		// Bootstrapped on 03/02/2025 using OP Deployer.
		VersionV180: common.HexToAddress("0x0a5bf8ebb4b177b2dcc6eba933db726a2e2e2b4d"),
		// Bootstrapped on 03/02/2025 using OP Deployer.
		VersionV200: common.HexToAddress("0x37739a6b0a3f1e7429499a4ec4a0685439daff5c"),
		// Bootstrapped on 04/03/2025 using OP Deployer.
		VersionV300: common.HexToAddress("0x2d56022cb84ce6b961c3b4288ca36386bcd9024c"),
		// Bootstrapped on 06/03/2025 using OP Deployer.
		VersionV400: common.HexToAddress("0x3246b056A3C9B14AA25b75ee133DC3747476DBc5"),
	},
}

func ValidatorAddress(chainID uint64, version string) (common.Address, error) {
	chainAddresses, ok := addresses[chainID]
	if !ok {
		return common.Address{}, fmt.Errorf("unsupported chain ID: %d", chainID)
	}

	address, ok := chainAddresses[version]
	if !ok {
		return common.Address{}, fmt.Errorf("unsupported version: %s", version)
	}
	return address, nil
}
