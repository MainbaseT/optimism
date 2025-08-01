package main

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/superchain"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-challenger/config"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
)

var (
	l1EthRpc                = "http://example.com:8545"
	l1Beacon                = "http://example.com:9000"
	gameFactoryAddressValue = "0xbb00000000000000000000000000000000000000"
	network                 = "op-mainnet"
	testNetwork             = "op-sepolia"
	l2EthRpc                = "http://example.com:9545"
	supervisorRpc           = "http://example.com/supervisor"
	cannonBin               = "./bin/cannon"
	cannonServer            = "./bin/op-program"
	cannonPreState          = "./pre.json"
	datadir                 = "./test_data"
	rollupRpc               = "http://example.com:8555"
	asteriscBin             = "./bin/asterisc"
	asteriscServer          = "./bin/op-program"
	asteriscPreState        = "./pre.json"
)

func TestLogLevel(t *testing.T) {
	t.Run("RejectInvalid", func(t *testing.T) {
		verifyArgsInvalid(t, "unknown level: foo", addRequiredArgs(types.TraceTypeAlphabet, "--log.level=foo"))
	})

	for _, lvl := range []string{"trace", "debug", "info", "error", "crit"} {
		lvl := lvl
		t.Run("AcceptValid_"+lvl, func(t *testing.T) {
			logger, _, err := dryRunWithArgs(addRequiredArgs(types.TraceTypeAlphabet, "--log.level", lvl))
			require.NoError(t, err)
			require.NotNil(t, logger)
		})
	}
}

func TestL2Experimental(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		url := "http://example.com:8888"
		cfg := configForArgs(t, addRequiredArgs(types.TraceTypeAlphabet, "--l2-experimental-eth-rpc="+url))
		require.Equal(t, url, cfg.Cannon.L2Experimental)
	})
}

func TestDefaultCLIOptionsMatchDefaultConfig(t *testing.T) {
	cfg := configForArgs(t, addRequiredArgs(types.TraceTypeAlphabet))
	defaultCfg := config.NewConfig(common.HexToAddress(gameFactoryAddressValue), l1EthRpc, l1Beacon, rollupRpc, l2EthRpc, datadir, types.TraceTypeAlphabet)
	require.Equal(t, defaultCfg, cfg)
}

func TestDefaultConfigIsValid(t *testing.T) {
	cfg := config.NewConfig(common.HexToAddress(gameFactoryAddressValue), l1EthRpc, l1Beacon, rollupRpc, l2EthRpc, datadir, types.TraceTypeAlphabet)
	require.NoError(t, cfg.Check())
}

func TestL1ETHRPCAddress(t *testing.T) {
	t.Run("Required", func(t *testing.T) {
		verifyArgsInvalid(t, "flag l1-eth-rpc is required", addRequiredArgsExcept(types.TraceTypeAlphabet, "--l1-eth-rpc"))
	})

	t.Run("Valid", func(t *testing.T) {
		url := "http://example.com:8888"
		cfg := configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--l1-eth-rpc", "--l1-eth-rpc="+url))
		require.Equal(t, url, cfg.L1EthRpc)
		require.Equal(t, url, cfg.TxMgrConfig.L1RPCURL)
	})
}

func TestL1Beacon(t *testing.T) {
	t.Run("Required", func(t *testing.T) {
		verifyArgsInvalid(t, "flag l1-beacon is required", addRequiredArgsExcept(types.TraceTypeAlphabet, "--l1-beacon"))
	})

	t.Run("Valid", func(t *testing.T) {
		url := "http://example.com:8888"
		cfg := configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--l1-beacon", "--l1-beacon="+url))
		require.Equal(t, url, cfg.L1Beacon)
	})
}

func TestOpSupervisor(t *testing.T) {
	t.Run("RequiredForSuperCannon", func(t *testing.T) {
		verifyArgsInvalid(t, "flag supervisor-rpc is required", addRequiredArgsExcept(types.TraceTypeSuperCannon, "--supervisor-rpc"))
	})
	t.Run("RequiredForSuperPermissioned", func(t *testing.T) {
		verifyArgsInvalid(t, "flag supervisor-rpc is required", addRequiredArgsExcept(types.TraceTypeSuperPermissioned, "--supervisor-rpc"))
	})
	t.Run("RequiredForSuperAsteriscKona", func(t *testing.T) {
		verifyArgsInvalid(t, "flag supervisor-rpc is required", addRequiredArgsExcept(types.TraceTypeSuperAsteriscKona, "--supervisor-rpc"))
	})

	for _, traceType := range types.TraceTypes {
		traceType := traceType
		if traceType == types.TraceTypeSuperCannon || traceType == types.TraceTypeSuperPermissioned || traceType == types.TraceTypeSuperAsteriscKona {
			continue
		}

		t.Run("NotRequiredForTraceType-"+traceType.String(), func(t *testing.T) {
			configForArgs(t, addRequiredArgsExcept(traceType, "--supervisor-rpc"))
		})
	}

	t.Run("Valid-SuperCannon", func(t *testing.T) {
		url := "http://localhost/supervisor"
		cfg := configForArgs(t, addRequiredArgsExcept(types.TraceTypeSuperCannon, "--supervisor-rpc", "--supervisor-rpc", url))
		require.Equal(t, url, cfg.SupervisorRPC)
	})

	t.Run("Valid-SuperPermissioned", func(t *testing.T) {
		url := "http://localhost/supervisor"
		cfg := configForArgs(t, addRequiredArgsExcept(types.TraceTypeSuperPermissioned, "--supervisor-rpc", "--supervisor-rpc", url))
		require.Equal(t, url, cfg.SupervisorRPC)
	})

	t.Run("Valid-SuperAsteriscKona", func(t *testing.T) {
		url := "http://localhost/supervisor"
		cfg := configForArgs(t, addRequiredArgsExcept(types.TraceTypeSuperAsteriscKona, "--supervisor-rpc", "--supervisor-rpc", url))
		require.Equal(t, url, cfg.SupervisorRPC)
	})
}

func TestTraceType(t *testing.T) {
	t.Run("Default", func(t *testing.T) {
		expectedDefault := []types.TraceType{types.TraceTypeCannon, types.TraceTypeAsteriscKona}
		cfg := configForArgs(t, addRequiredArgsForMultipleTracesExcept(expectedDefault, "--trace-type"))
		require.Equal(t, expectedDefault, cfg.TraceTypes)
	})

	for _, traceType := range types.TraceTypes {
		traceType := traceType
		t.Run("Valid_"+traceType.String(), func(t *testing.T) {
			cfg := configForArgs(t, addRequiredArgs(traceType))
			require.Equal(t, []types.TraceType{traceType}, cfg.TraceTypes)
		})
	}

	t.Run("Invalid", func(t *testing.T) {
		verifyArgsInvalid(t, "unknown trace type: \"foo\"", addRequiredArgsExcept(types.TraceTypeAlphabet, "--trace-type", "--trace-type=foo"))
	})
}

func TestMultipleTraceTypes(t *testing.T) {
	t.Run("WithAllOptions", func(t *testing.T) {
		argsMap := requiredArgs(types.TraceTypeCannon)
		// Add Asterisc required flags
		addRequiredAsteriscArgs(argsMap)
		args := toArgList(argsMap)
		// Add extra trace types (cannon is already specified)
		args = append(args,
			"--trace-type", types.TraceTypeAlphabet.String())
		args = append(args,
			"--trace-type", types.TraceTypePermissioned.String())
		args = append(args,
			"--trace-type", types.TraceTypeAsterisc.String())
		cfg := configForArgs(t, args)
		require.Equal(t, []types.TraceType{types.TraceTypeCannon, types.TraceTypeAlphabet, types.TraceTypePermissioned, types.TraceTypeAsterisc}, cfg.TraceTypes)
	})
	t.Run("WithSomeOptions", func(t *testing.T) {
		argsMap := requiredArgs(types.TraceTypeCannon)
		args := toArgList(argsMap)
		// Add extra trace types (cannon is already specified)
		args = append(args,
			"--trace-type", types.TraceTypeAlphabet.String())
		cfg := configForArgs(t, args)
		require.Equal(t, []types.TraceType{types.TraceTypeCannon, types.TraceTypeAlphabet}, cfg.TraceTypes)
	})

	t.Run("SpecifySameOptionMultipleTimes", func(t *testing.T) {
		argsMap := requiredArgs(types.TraceTypeCannon)
		args := toArgList(argsMap)
		// Add cannon trace type again
		args = append(args, "--trace-type", types.TraceTypeCannon.String())
		// We're fine with the same option being listed multiple times, just deduplicate them.
		cfg := configForArgs(t, args)
		require.Equal(t, []types.TraceType{types.TraceTypeCannon}, cfg.TraceTypes)
	})
}

func TestGameFactoryAddress(t *testing.T) {
	t.Run("RequiredWhenNetworkNotSupplied", func(t *testing.T) {
		verifyArgsInvalid(t, "flag game-factory-address or network is required", addRequiredArgsExcept(types.TraceTypeAlphabet, "--game-factory-address"))
	})

	t.Run("RequiredWhenMultipleNetworksSuppliedWithDifferentFactories", func(t *testing.T) {
		verifyArgsInvalid(t, "specified networks use different dispute game factories, flag game-factory-address required", addRequiredArgsExcept(types.TraceTypeAlphabet, "--game-factory-address", "--network", "op-sepolia,op-mainnet"))
	})

	t.Run("Valid", func(t *testing.T) {
		addr := common.Address{0xbb, 0xcc, 0xdd}
		cfg := configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--game-factory-address", "--game-factory-address="+addr.Hex()))
		require.Equal(t, addr, cfg.GameFactoryAddress)
	})

	t.Run("Invalid", func(t *testing.T) {
		verifyArgsInvalid(t, "invalid address: foo", addRequiredArgsExcept(types.TraceTypeAlphabet, "--game-factory-address", "--game-factory-address=foo"))
	})

	t.Run("OverridesNetwork", func(t *testing.T) {
		addr := common.Address{0xbb, 0xcc, 0xdd}
		cfg := configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--game-factory-address", "--game-factory-address", addr.Hex(), "--network", "op-sepolia"))
		require.Equal(t, addr, cfg.GameFactoryAddress)
	})
}

func TestNetwork(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		opSepoliaChainId := uint64(11155420)
		opSepolia, err := superchain.GetChain(opSepoliaChainId)
		require.NoError(t, err)
		opSepoliaCfg, err := opSepolia.Config()
		require.NoError(t, err)
		cfg := configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--game-factory-address", "--network=op-sepolia"))
		require.EqualValues(t, *opSepoliaCfg.Addresses.DisputeGameFactoryProxy, cfg.GameFactoryAddress)
	})

	t.Run("UnknownNetwork", func(t *testing.T) {
		verifyArgsInvalid(t, "unknown chain: not-a-network", addRequiredArgsExcept(types.TraceTypeAlphabet, "--game-factory-address", "--network=not-a-network"))
	})

	t.Run("ChainIDAllowedWhenGameFactoryAddressSupplied", func(t *testing.T) {
		addr := common.Address{0xbb, 0xcc, 0xdd}
		cfg := configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--game-factory-address", "--network=1234", "--game-factory-address="+addr.Hex()))
		require.Equal(t, addr, cfg.GameFactoryAddress)
		require.Equal(t, []string{"1234"}, cfg.Cannon.Networks)
	})
}

func TestGameAllowlist(t *testing.T) {
	t.Run("Optional", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--game-allowlist"))
		require.NoError(t, cfg.Check())
	})

	t.Run("Valid", func(t *testing.T) {
		addr := common.Address{0xbb, 0xcc, 0xdd}
		cfg := configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--game-allowlist", "--game-allowlist="+addr.Hex()))
		require.Contains(t, cfg.GameAllowlist, addr)
	})

	t.Run("Invalid", func(t *testing.T) {
		verifyArgsInvalid(t, "invalid address: foo", addRequiredArgsExcept(types.TraceTypeAlphabet, "--game-allowlist", "--game-allowlist=foo"))
	})
}

func TestTxManagerFlagsSupported(t *testing.T) {
	// Not a comprehensive list of flags, just enough to sanity check the txmgr.CLIFlags were defined
	cfg := configForArgs(t, addRequiredArgs(types.TraceTypeAlphabet, "--"+txmgr.NumConfirmationsFlagName, "7"))
	require.Equal(t, uint64(7), cfg.TxMgrConfig.NumConfirmations)
}

func TestMaxConcurrency(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		expected := uint(345)
		cfg := configForArgs(t, addRequiredArgs(types.TraceTypeAlphabet, "--max-concurrency", "345"))
		require.Equal(t, expected, cfg.MaxConcurrency)
	})

	t.Run("Invalid", func(t *testing.T) {
		verifyArgsInvalid(
			t,
			"invalid value \"abc\" for flag -max-concurrency",
			addRequiredArgs(types.TraceTypeAlphabet, "--max-concurrency", "abc"))
	})

	t.Run("Zero", func(t *testing.T) {
		verifyArgsInvalid(
			t,
			"max-concurrency must not be 0",
			addRequiredArgs(types.TraceTypeAlphabet, "--max-concurrency", "0"))
	})
}

func TestMaxPendingTx(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		expected := uint64(345)
		cfg := configForArgs(t, addRequiredArgs(types.TraceTypeAlphabet, "--max-pending-tx", "345"))
		require.Equal(t, expected, cfg.MaxPendingTx)
	})

	t.Run("Zero", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs(types.TraceTypeAlphabet, "--max-pending-tx", "0"))
		require.Equal(t, uint64(0), cfg.MaxPendingTx)
	})

	t.Run("Invalid", func(t *testing.T) {
		verifyArgsInvalid(
			t,
			"invalid value \"abc\" for flag -max-pending-tx",
			addRequiredArgs(types.TraceTypeAlphabet, "--max-pending-tx", "abc"))
	})
}

func TestPollInterval(t *testing.T) {
	t.Run("UsesDefault", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs(types.TraceTypeCannon))
		require.Equal(t, config.DefaultPollInterval, cfg.PollInterval)
	})

	t.Run("Valid", func(t *testing.T) {
		expected := 100 * time.Second
		cfg := configForArgs(t, addRequiredArgs(types.TraceTypeAlphabet, "--http-poll-interval", "100s"))
		require.Equal(t, expected, cfg.PollInterval)
	})

	t.Run("Invalid", func(t *testing.T) {
		verifyArgsInvalid(
			t,
			"invalid value \"abc\" for flag -http-poll-interval",
			addRequiredArgs(types.TraceTypeAlphabet, "--http-poll-interval", "abc"))
	})
}

func TestAsteriscOpProgramRequiredArgs(t *testing.T) {
	traceType := types.TraceTypeAsterisc
	t.Run(fmt.Sprintf("TestAsteriscServer-%v", traceType), func(t *testing.T) {
		t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
			configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--asterisc-server"))
		})

		t.Run("Required", func(t *testing.T) {
			verifyArgsInvalid(t, "flag asterisc-server is required", addRequiredArgsExcept(traceType, "--asterisc-server"))
		})

		t.Run("Valid", func(t *testing.T) {
			cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--asterisc-server", "--asterisc-server=./op-program"))
			require.Equal(t, "./op-program", cfg.Asterisc.Server)
		})
	})

	t.Run(fmt.Sprintf("TestAsteriscAbsolutePrestate-%v", traceType), func(t *testing.T) {
		t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
			configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--asterisc-prestate"))
		})

		t.Run("Required", func(t *testing.T) {
			verifyArgsInvalid(t, "flag prestates-url/asterisc-prestates-url or asterisc-prestate is required", addRequiredArgsExcept(traceType, "--asterisc-prestate"))
		})

		t.Run("Valid", func(t *testing.T) {
			cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--asterisc-prestate", "--asterisc-prestate=./pre.json"))
			require.Equal(t, "./pre.json", cfg.AsteriscAbsolutePreState)
		})
	})

	t.Run(fmt.Sprintf("TestPrestateBaseURL-%v", traceType), func(t *testing.T) {
		allPrestateOptions := []string{"--prestates-url", "--asterisc-prestates-url", "--asterisc-prestate"}
		t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
			configForArgs(t, addRequiredArgsExceptArr(types.TraceTypeAlphabet, allPrestateOptions))
		})

		t.Run("NotRequiredIfAsteriscPrestatesBaseURLSet", func(t *testing.T) {
			configForArgs(t, addRequiredArgsExceptArr(traceType, allPrestateOptions, "--asterisc-prestates-url=http://localhost/foo"))
		})

		t.Run("AsteriscPrestatesBaseURLTakesPrecedence", func(t *testing.T) {
			cfg := configForArgs(t, addRequiredArgsExceptArr(traceType, allPrestateOptions, "--asterisc-prestates-url=http://localhost/foo", "--prestates-url=http://localhost/bar"))
			require.Equal(t, "http://localhost/foo", cfg.AsteriscAbsolutePreStateBaseURL.String())
		})

		t.Run("RequiredIfAsteriscPrestatesBaseURLNotSet", func(t *testing.T) {
			verifyArgsInvalid(t, "flag prestates-url/asterisc-prestates-url or asterisc-prestate is required", addRequiredArgsExceptArr(traceType, allPrestateOptions))
		})

		t.Run("Invalid", func(t *testing.T) {
			verifyArgsInvalid(t, "invalid prestates-url (:foo/bar)", addRequiredArgsExceptArr(traceType, allPrestateOptions, "--prestates-url=:foo/bar"))
		})

		t.Run("Valid", func(t *testing.T) {
			cfg := configForArgs(t, addRequiredArgsExceptArr(traceType, allPrestateOptions, "--prestates-url=http://localhost/foo"))
			require.Equal(t, "http://localhost/foo", cfg.AsteriscAbsolutePreStateBaseURL.String())
		})
	})

	t.Run(fmt.Sprintf("TestAsteriscAbsolutePrestateBaseURL-%v", traceType), func(t *testing.T) {
		t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
			configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--asterisc-prestates-url"))
		})

		t.Run("Required", func(t *testing.T) {
			verifyArgsInvalid(t, "flag prestates-url/asterisc-prestates-url or asterisc-prestate is required", addRequiredArgsExcept(traceType, "--asterisc-prestate"))
		})

		t.Run("Valid", func(t *testing.T) {
			cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--asterisc-prestates-url", "--asterisc-prestates-url=http://localhost/bar"))
			require.Equal(t, "http://localhost/bar", cfg.AsteriscAbsolutePreStateBaseURL.String())
		})
	})
}

func TestAsteriscKonaRequiredArgs(t *testing.T) {
	traceType := types.TraceTypeAsteriscKona
	t.Run(fmt.Sprintf("TestAsteriscServer-%v", traceType), func(t *testing.T) {
		t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
			configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--asterisc-kona-server"))
		})

		t.Run("Required", func(t *testing.T) {
			verifyArgsInvalid(t, "flag asterisc-kona-server is required", addRequiredArgsExcept(traceType, "--asterisc-kona-server"))
		})

		t.Run("Valid", func(t *testing.T) {
			cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--asterisc-kona-server", "--asterisc-kona-server=./kona-host"))
			require.Equal(t, "./kona-host", cfg.AsteriscKona.Server)
		})
	})

	t.Run(fmt.Sprintf("TestAsteriscAbsolutePrestate-%v", traceType), func(t *testing.T) {
		t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
			configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--asterisc-kona-prestate"))
		})

		t.Run("Required", func(t *testing.T) {
			verifyArgsInvalid(t, "flag prestates-url/asterisc-kona-prestates-url or asterisc-kona-prestate is required", addRequiredArgsExcept(traceType, "--asterisc-kona-prestate"))
		})

		t.Run("Valid", func(t *testing.T) {
			cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--asterisc-kona-prestate", "--asterisc-kona-prestate=./pre.json"))
			require.Equal(t, "./pre.json", cfg.AsteriscKonaAbsolutePreState)
		})
	})

	t.Run(fmt.Sprintf("TestAsteriscAbsolutePrestateBaseURL-%v", traceType), func(t *testing.T) {
		t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
			configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--asterisc-kona-prestates-url"))
		})

		t.Run("Required", func(t *testing.T) {
			verifyArgsInvalid(t, "flag prestates-url/asterisc-kona-prestates-url or asterisc-kona-prestate is required", addRequiredArgsExcept(traceType, "--asterisc-kona-prestate"))
		})

		t.Run("Valid", func(t *testing.T) {
			cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--asterisc-kona-prestates-url", "--asterisc-kona-prestates-url=http://localhost/bar"))
			require.Equal(t, "http://localhost/bar", cfg.AsteriscKonaAbsolutePreStateBaseURL.String())
		})
	})

	t.Run(fmt.Sprintf("TestPrestateBaseURL-%v", traceType), func(t *testing.T) {
		allPrestateOptions := []string{"--prestates-url", "--asterisc-kona-prestates-url", "--asterisc-kona-prestate"}
		t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
			configForArgs(t, addRequiredArgsExceptArr(types.TraceTypeAlphabet, allPrestateOptions))
		})

		t.Run("NotRequiredIfAsteriscKonaPrestatesBaseURLSet", func(t *testing.T) {
			configForArgs(t, addRequiredArgsExceptArr(traceType, allPrestateOptions, "--asterisc-kona-prestates-url=http://localhost/foo"))
		})

		t.Run("AsteriscKonaPrestatesBaseURLTakesPrecedence", func(t *testing.T) {
			cfg := configForArgs(t, addRequiredArgsExceptArr(traceType, allPrestateOptions, "--asterisc-kona-prestates-url=http://localhost/foo", "--prestates-url=http://localhost/bar"))
			require.Equal(t, "http://localhost/foo", cfg.AsteriscKonaAbsolutePreStateBaseURL.String())
		})

		t.Run("RequiredIfAsteriscKonaPrestatesBaseURLNotSet", func(t *testing.T) {
			verifyArgsInvalid(t, "flag prestates-url/asterisc-kona-prestates-url or asterisc-kona-prestate is required", addRequiredArgsExceptArr(traceType, allPrestateOptions))
		})

		t.Run("Invalid", func(t *testing.T) {
			verifyArgsInvalid(t, "invalid prestates-url (:foo/bar)", addRequiredArgsExceptArr(traceType, allPrestateOptions, "--prestates-url=:foo/bar"))
		})

		t.Run("Valid", func(t *testing.T) {
			cfg := configForArgs(t, addRequiredArgsExceptArr(traceType, allPrestateOptions, "--prestates-url=http://localhost/foo"))
			require.Equal(t, "http://localhost/foo", cfg.AsteriscKonaAbsolutePreStateBaseURL.String())
		})
	})
}

func TestAsteriscBaseRequiredArgs(t *testing.T) {
	for _, traceType := range []types.TraceType{types.TraceTypeAsterisc, types.TraceTypeAsteriscKona} {
		traceType := traceType
		t.Run(fmt.Sprintf("TestAsteriscBin-%v", traceType), func(t *testing.T) {
			t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--asterisc-bin"))
			})

			t.Run("Required", func(t *testing.T) {
				verifyArgsInvalid(t, "flag asterisc-bin is required", addRequiredArgsExcept(traceType, "--asterisc-bin"))
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--asterisc-bin", "--asterisc-bin=./asterisc"))
				require.Equal(t, "./asterisc", cfg.Asterisc.VmBin)
			})
		})

		t.Run(fmt.Sprintf("TestL2Rpc-%v", traceType), func(t *testing.T) {
			t.Run("RequiredForAsteriscTrace", func(t *testing.T) {
				verifyArgsInvalid(t, "flag l2-eth-rpc is required", addRequiredArgsExcept(traceType, "--l2-eth-rpc"))
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgs(traceType))
				require.Equal(t, []string{l2EthRpc}, cfg.L2Rpcs)
			})
		})

		t.Run(fmt.Sprintf("TestAsteriscSnapshotFreq-%v", traceType), func(t *testing.T) {
			t.Run("UsesDefault", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgs(traceType))
				require.Equal(t, config.DefaultAsteriscSnapshotFreq, cfg.Asterisc.SnapshotFreq)
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgs(traceType, "--asterisc-snapshot-freq=1234"))
				require.Equal(t, uint(1234), cfg.Asterisc.SnapshotFreq)
			})

			t.Run("Invalid", func(t *testing.T) {
				verifyArgsInvalid(t, "invalid value \"abc\" for flag -asterisc-snapshot-freq",
					addRequiredArgs(traceType, "--asterisc-snapshot-freq=abc"))
			})
		})

		t.Run(fmt.Sprintf("TestAsteriscInfoFreq-%v", traceType), func(t *testing.T) {
			t.Run("UsesDefault", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgs(traceType))
				require.Equal(t, config.DefaultAsteriscInfoFreq, cfg.Asterisc.InfoFreq)
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgs(traceType, "--asterisc-info-freq=1234"))
				require.Equal(t, uint(1234), cfg.Asterisc.InfoFreq)
			})

			t.Run("Invalid", func(t *testing.T) {
				verifyArgsInvalid(t, "invalid value \"abc\" for flag -asterisc-info-freq",
					addRequiredArgs(traceType, "--asterisc-info-freq=abc"))
			})
		})

		t.Run(fmt.Sprintf("TestRequireEitherNetworkOrRollupAndGenesis-%v", traceType), func(t *testing.T) {
			verifyArgsInvalid(
				t,
				fmt.Sprintf("flag network or rollup-config/%s-rollup-config and l2-genesis/%s-l2-genesis is required", traceType, traceType),
				addRequiredArgsExcept(traceType, "--network"))
			verifyArgsInvalid(
				t,
				fmt.Sprintf("flag network or rollup-config/%s-rollup-config and l2-genesis/%s-l2-genesis is required", traceType, traceType),
				addRequiredArgsExcept(traceType, "--network", "--rollup-config=rollup.json"))
			verifyArgsInvalid(
				t,
				fmt.Sprintf("flag network or rollup-config/%s-rollup-config and l2-genesis/%s-l2-genesis is required", traceType, traceType),
				addRequiredArgsExcept(traceType, "--network", "--l2-genesis=gensis.json"))
		})

		t.Run(fmt.Sprintf("TestMustNotSpecifyNetworkAndRollup-%v", traceType), func(t *testing.T) {
			verifyArgsInvalid(
				t,
				"flag network can not be used with rollup-config, l2-genesis or asterisc-kona-l2-custom",
				addRequiredArgs(traceType, "--rollup-config=rollup.json"))
		})

		t.Run(fmt.Sprintf("TestNetwork-%v", traceType), func(t *testing.T) {
			t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--network"))
			})

			t.Run("NotRequiredWhenRollupAndGenesisSpecified", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(traceType, "--network",
					"--rollup-config=rollup.json", "--l2-genesis=genesis.json"))
			})

			t.Run("NotRequiredWhenNetworkSpecified", func(t *testing.T) {
				args := requiredArgs(traceType)
				delete(args, "--network")
				delete(args, "--game-factory-address")
				args["--network"] = "op-sepolia"
				cfg := configForArgs(t, toArgList(args))
				require.Equal(t, []string{"op-sepolia"}, cfg.Asterisc.Networks)
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--network", "--network", testNetwork))
				require.Equal(t, []string{testNetwork}, cfg.Asterisc.Networks)
			})
		})

		t.Run(fmt.Sprintf("TestAsteriscRollupConfig-%v", traceType), func(t *testing.T) {
			t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--asterisc-rollup-config"))
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--network", "--rollup-config=rollup.json", "--l2-genesis=genesis.json"))
				require.Equal(t, []string{"rollup.json"}, cfg.Asterisc.RollupConfigPaths)
			})
		})

		t.Run(fmt.Sprintf("TestL2Genesis-%v", traceType), func(t *testing.T) {
			t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--l2-genesis"))
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--network", "--rollup-config=rollup.json", "--l2-genesis=genesis.json"))
				require.Equal(t, []string{"genesis.json"}, cfg.Asterisc.L2GenesisPaths)
			})
		})
	}
}

func TestAlphabetRequiredArgs(t *testing.T) {
	t.Run(fmt.Sprintf("TestL2Rpc-%v", types.TraceTypeAlphabet), func(t *testing.T) {
		t.Run("RequiredForAlphabetTrace", func(t *testing.T) {
			verifyArgsInvalid(t, "flag l2-eth-rpc is required", addRequiredArgsExcept(types.TraceTypeAlphabet, "--l2-eth-rpc"))
		})

		t.Run("Valid", func(t *testing.T) {
			cfg := configForArgs(t, addRequiredArgs(types.TraceTypeAlphabet))
			require.Equal(t, []string{l2EthRpc}, cfg.L2Rpcs)
		})
	})
}

func TestCannonCustomConfigArgs(t *testing.T) {
	for _, traceType := range []types.TraceType{types.TraceTypeCannon, types.TraceTypePermissioned} {
		traceType := traceType

		t.Run(fmt.Sprintf("TestRequireEitherCannonNetworkOrRollupAndGenesis-%v", traceType), func(t *testing.T) {
			verifyArgsInvalid(
				t,
				"flag network or rollup-config/cannon-rollup-config and l2-genesis/cannon-l2-genesis is required",
				addRequiredArgsExcept(traceType, "--network"))
			verifyArgsInvalid(
				t,
				"flag network or rollup-config/cannon-rollup-config and l2-genesis/cannon-l2-genesis is required",
				addRequiredArgsExcept(traceType, "--network", "--cannon-rollup-config=rollup.json"))
			verifyArgsInvalid(
				t,
				"flag network or rollup-config/cannon-rollup-config and l2-genesis/cannon-l2-genesis is required",
				addRequiredArgsExcept(traceType, "--network", "--cannon-l2-genesis=gensis.json"))
		})

		t.Run(fmt.Sprintf("TestMustNotSpecifyNetworkAndRollup-%v", traceType), func(t *testing.T) {
			verifyArgsInvalid(
				t,
				"flag network can not be used with cannon-rollup-config, l2-genesis or cannon-l2-custom",
				addRequiredArgs(traceType, "--cannon-rollup-config=rollup.json"))
		})

		t.Run(fmt.Sprintf("TestMustNotSpecifyNetworkAndRollup-%v", traceType), func(t *testing.T) {
			args := requiredArgs(traceType)
			delete(args, "--network")
			delete(args, "--game-factory-address")
			args["--network"] = network
			args["--cannon-rollup-config"] = "rollup.json"
			args["--cannon-l2-genesis"] = "gensis.json"
			args["--cannon-l2-custom"] = "true"
			verifyArgsInvalid(
				t,
				"flag network can not be used with cannon-rollup-config, cannon-l2-genesis or cannon-l2-custom",
				toArgList(args))
		})

		t.Run(fmt.Sprintf("TestNetwork-%v", traceType), func(t *testing.T) {
			t.Run("NotRequiredWhenRollupAndGenesIsSpecified", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(traceType, "--network",
					"--cannon-rollup-config=rollup.json", "--cannon-l2-genesis=genesis.json"))
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--network", "--network", testNetwork))
				require.Equal(t, []string{testNetwork}, cfg.Cannon.Networks)
			})
		})

		t.Run(fmt.Sprintf("TestSetCannonL2ChainId-%v", traceType), func(t *testing.T) {
			cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--network",
				"--cannon-rollup-config=rollup.json",
				"--cannon-l2-genesis=genesis.json",
				"--cannon-l2-custom"))
			require.True(t, cfg.Cannon.L2Custom)
		})

		t.Run(fmt.Sprintf("TestCannonRollupConfig-%v", traceType), func(t *testing.T) {
			t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--cannon-rollup-config"))
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--network", "--cannon-rollup-config=rollup.json", "--cannon-l2-genesis=genesis.json"))
				require.Equal(t, []string{"rollup.json"}, cfg.Cannon.RollupConfigPaths)
			})
		})

		t.Run(fmt.Sprintf("TestCannonL2Genesis-%v", traceType), func(t *testing.T) {
			t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--cannon-l2-genesis"))
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--network", "--cannon-rollup-config=rollup.json", "--cannon-l2-genesis=genesis.json"))
				require.Equal(t, []string{"genesis.json"}, cfg.Cannon.L2GenesisPaths)
			})
		})
	}
}

func TestSuperCannonCustomConfigArgs(t *testing.T) {
	for _, traceType := range []types.TraceType{types.TraceTypeSuperCannon, types.TraceTypeSuperPermissioned} {
		traceType := traceType

		t.Run(fmt.Sprintf("TestRequireEitherCannonNetworkOrRollupAndGenesisAndDepset-%v", traceType), func(t *testing.T) {
			expectedErrorMessage := "flag network or rollup-config/cannon-rollup-config, l2-genesis/cannon-l2-genesis and depset-config/cannon-depset-config is required"
			// Missing all
			verifyArgsInvalid(
				t,
				expectedErrorMessage,
				addRequiredArgsExcept(traceType, "--network"))
			// Missing l2-genesis
			verifyArgsInvalid(
				t,
				expectedErrorMessage,
				addRequiredArgsExcept(traceType, "--network", "--cannon-rollup-config=rollup.json", "--cannon-depset-config=depset.json"))
			// Missing rollup-config
			verifyArgsInvalid(
				t,
				expectedErrorMessage,
				addRequiredArgsExcept(traceType, "--network", "--cannon-l2-genesis=gensis.json", "--cannon-depset-config=depset.json"))
			// Missing depset-config
			verifyArgsInvalid(
				t,
				expectedErrorMessage,
				addRequiredArgsExcept(traceType, "--network", "--cannon-rollup-config=rollup.json", "--cannon-l2-genesis=gensis.json"))
		})

		t.Run(fmt.Sprintf("TestMustNotSpecifyNetworkAndRollup-%v", traceType), func(t *testing.T) {
			verifyArgsInvalid(
				t,
				"flag network can not be used with cannon-rollup-config, l2-genesis or cannon-l2-custom",
				addRequiredArgs(traceType, "--cannon-rollup-config=rollup.json"))
		})

		t.Run(fmt.Sprintf("TestMustNotSpecifyNetworkAndRollup-%v", traceType), func(t *testing.T) {
			args := requiredArgs(traceType)
			delete(args, "--network")
			delete(args, "--game-factory-address")
			args["--network"] = network
			args["--cannon-rollup-config"] = "rollup.json"
			args["--cannon-l2-genesis"] = "gensis.json"
			args["--cannon-l2-custom"] = "true"
			verifyArgsInvalid(
				t,
				"flag network can not be used with cannon-rollup-config, cannon-l2-genesis or cannon-l2-custom",
				toArgList(args))
		})

		t.Run(fmt.Sprintf("TestNetwork-%v", traceType), func(t *testing.T) {
			t.Run("NotRequiredWhenRollupGenesisAndDepsetIsSpecified", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(traceType, "--network",
					"--cannon-rollup-config=rollup.json", "--cannon-l2-genesis=genesis.json", "--cannon-depset-config=depset.json"))
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--network", "--network", testNetwork))
				require.Equal(t, []string{testNetwork}, cfg.Cannon.Networks)
			})
		})

		t.Run(fmt.Sprintf("TestSetCannonL2ChainId-%v", traceType), func(t *testing.T) {
			cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--network",
				"--cannon-rollup-config=rollup.json",
				"--cannon-l2-genesis=genesis.json",
				"--cannon-depset-config=depset.json",
				"--cannon-l2-custom"))
			require.True(t, cfg.Cannon.L2Custom)
		})

		t.Run(fmt.Sprintf("TestCannonRollupConfig-%v", traceType), func(t *testing.T) {
			t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--cannon-rollup-config"))
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--network",
					"--cannon-rollup-config=rollup.json", "--cannon-l2-genesis=genesis.json", "--cannon-depset-config=depset.json"))
				require.Equal(t, []string{"rollup.json"}, cfg.Cannon.RollupConfigPaths)
			})
		})

		t.Run(fmt.Sprintf("TestCannonL2Genesis-%v", traceType), func(t *testing.T) {
			t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--cannon-l2-genesis"))
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--network", "--cannon-rollup-config=rollup.json", "--cannon-l2-genesis=genesis.json", "--cannon-depset-config=depset.json"))
				require.Equal(t, []string{"genesis.json"}, cfg.Cannon.L2GenesisPaths)
			})
		})

		t.Run(fmt.Sprintf("TestCannonDepsetConfig-%v", traceType), func(t *testing.T) {
			t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--cannon-depset-config"))
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--network", "--cannon-rollup-config=rollup.json", "--cannon-l2-genesis=genesis.json", "--cannon-depset-config=depset.json"))
				require.Equal(t, "depset.json", cfg.Cannon.DepsetConfigPath)
			})
		})
	}
}

func TestSuperAsteriscKonaCustomConfigArgs(t *testing.T) {
	for _, traceType := range []types.TraceType{types.TraceTypeSuperAsteriscKona} {
		traceType := traceType

		t.Run(fmt.Sprintf("TestRequireEitherAsteriscKonaNetworkOrRollupAndGenesisAndDepset-%v", traceType), func(t *testing.T) {
			expectedErrorMessage := "flag network or rollup-config/asterisc-kona-rollup-config, l2-genesis/asterisc-kona-l2-genesis and depset-config/asterisc-kona-depset-config is required"
			// Missing all
			verifyArgsInvalid(
				t,
				expectedErrorMessage,
				addRequiredArgsExcept(traceType, "--network"))
			// Missing l2-genesis
			verifyArgsInvalid(
				t,
				expectedErrorMessage,
				addRequiredArgsExcept(traceType, "--network", "--asterisc-kona-rollup-config=rollup.json", "--asterisc-kona-depset-config=depset.json"))
			// Missing rollup-config
			verifyArgsInvalid(
				t,
				expectedErrorMessage,
				addRequiredArgsExcept(traceType, "--network", "--asterisc-kona-l2-genesis=gensis.json", "--asterisc-kona-depset-config=depset.json"))
			// Missing depset-config
			verifyArgsInvalid(
				t,
				expectedErrorMessage,
				addRequiredArgsExcept(traceType, "--network", "--asterisc-kona-rollup-config=rollup.json", "--asterisc-kona-l2-genesis=gensis.json"))
		})

		t.Run(fmt.Sprintf("TestMustNotSpecifyNetworkAndRollup-%v", traceType), func(t *testing.T) {
			verifyArgsInvalid(
				t,
				"flag network can not be used with asterisc-kona-rollup-config, l2-genesis or asterisc-kona-l2-custom",
				addRequiredArgs(traceType, "--asterisc-kona-rollup-config=rollup.json"))
		})

		t.Run(fmt.Sprintf("TestMustNotSpecifyNetworkAndRollup-%v", traceType), func(t *testing.T) {
			args := requiredArgs(traceType)
			delete(args, "--network")
			delete(args, "--game-factory-address")
			args["--network"] = network
			args["--asterisc-kona-rollup-config"] = "rollup.json"
			args["--asterisc-kona-l2-genesis"] = "gensis.json"
			args["--asterisc-kona-l2-custom"] = "true"
			verifyArgsInvalid(
				t,
				"flag network can not be used with asterisc-kona-rollup-config, asterisc-kona-l2-genesis or asterisc-kona-l2-custom",
				toArgList(args))
		})

		t.Run(fmt.Sprintf("TestNetwork-%v", traceType), func(t *testing.T) {
			t.Run("NotRequiredWhenRollupGenesisAndDepsetIsSpecified", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(traceType, "--network",
					"--asterisc-kona-rollup-config=rollup.json", "--asterisc-kona-l2-genesis=genesis.json", "--asterisc-kona-depset-config=depset.json"))
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--network", "--network", testNetwork))
				require.Equal(t, []string{testNetwork}, cfg.AsteriscKona.Networks)
			})
		})

		t.Run(fmt.Sprintf("TestSetAsteriscL2ChainId-%v", traceType), func(t *testing.T) {
			cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--network",
				"--asterisc-kona-rollup-config=rollup.json",
				"--asterisc-kona-l2-genesis=genesis.json",
				"--asterisc-kona-depset-config=depset.json",
				"--asterisc-kona-l2-custom"))
			require.True(t, cfg.AsteriscKona.L2Custom)
		})

		t.Run(fmt.Sprintf("TestAsteriscRollupConfig-%v", traceType), func(t *testing.T) {
			t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--asterisc-kona-rollup-config"))
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--network",
					"--asterisc-kona-rollup-config=rollup.json", "--asterisc-kona-l2-genesis=genesis.json", "--asterisc-kona-depset-config=depset.json"))
				require.Equal(t, []string{"rollup.json"}, cfg.AsteriscKona.RollupConfigPaths)
			})
		})

		t.Run(fmt.Sprintf("TestAsteriscL2Genesis-%v", traceType), func(t *testing.T) {
			t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--asterisc-kona-l2-genesis"))
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--network", "--asterisc-kona-rollup-config=rollup.json", "--asterisc-kona-l2-genesis=genesis.json", "--asterisc-kona-depset-config=depset.json"))
				require.Equal(t, []string{"genesis.json"}, cfg.AsteriscKona.L2GenesisPaths)
			})
		})

		t.Run(fmt.Sprintf("TestAsteriscDepsetConfig-%v", traceType), func(t *testing.T) {
			t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--asterisc-kona-depset-config"))
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--network", "--asterisc-kona-rollup-config=rollup.json", "--asterisc-kona-l2-genesis=genesis.json", "--asterisc-kona-depset-config=depset.json"))
				require.Equal(t, "depset.json", cfg.AsteriscKona.DepsetConfigPath)
			})
		})
	}
}

func TestCannonRequiredArgs(t *testing.T) {
	for _, traceType := range []types.TraceType{types.TraceTypeCannon, types.TraceTypePermissioned, types.TraceTypeSuperCannon, types.TraceTypeSuperPermissioned} {
		traceType := traceType
		t.Run(fmt.Sprintf("TestCannonBin-%v", traceType), func(t *testing.T) {
			t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--cannon-bin"))
			})

			t.Run("Required", func(t *testing.T) {
				verifyArgsInvalid(t, "flag cannon-bin is required", addRequiredArgsExcept(traceType, "--cannon-bin"))
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--cannon-bin", "--cannon-bin=./cannon"))
				require.Equal(t, "./cannon", cfg.Cannon.VmBin)
			})
		})

		t.Run(fmt.Sprintf("TestCannonServer-%v", traceType), func(t *testing.T) {
			t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--cannon-server"))
			})

			t.Run("Required", func(t *testing.T) {
				verifyArgsInvalid(t, "flag cannon-server is required", addRequiredArgsExcept(traceType, "--cannon-server"))
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--cannon-server", "--cannon-server=./op-program"))
				require.Equal(t, "./op-program", cfg.Cannon.Server)
			})
		})

		t.Run(fmt.Sprintf("TestCannonAbsolutePrestate-%v", traceType), func(t *testing.T) {
			t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--cannon-prestate"))
			})

			t.Run("Required", func(t *testing.T) {
				verifyArgsInvalid(t, "flag prestates-url/cannon-prestates-url or cannon-prestate is required", addRequiredArgsExcept(traceType, "--cannon-prestate"))
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--cannon-prestate", "--cannon-prestate=./pre.json"))
				require.Equal(t, "./pre.json", cfg.CannonAbsolutePreState)
			})
		})

		t.Run(fmt.Sprintf("TestCannonPrestatesBaseURL-%v", traceType), func(t *testing.T) {
			t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--cannon-prestates-url"))
			})

			t.Run("Required", func(t *testing.T) {
				verifyArgsInvalid(t, "flag prestates-url/cannon-prestates-url or cannon-prestate is required", addRequiredArgsExcept(traceType, "--cannon-prestate"))
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--cannon-prestates-url", "--cannon-prestates-url=http://localhost/foo"))
				require.Equal(t, "http://localhost/foo", cfg.CannonAbsolutePreStateBaseURL.String())
			})
		})

		t.Run(fmt.Sprintf("TestPrestateBaseURL-%v", traceType), func(t *testing.T) {
			allPrestateOptions := []string{"--prestates-url", "--cannon-prestates-url", "--cannon-prestate"}
			t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExceptArr(types.TraceTypeAlphabet, allPrestateOptions))
			})

			t.Run("NotRequiredIfCannonPrestatesBaseURLSet", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExceptArr(traceType, allPrestateOptions, "--cannon-prestates-url=http://localhost/foo"))
			})

			t.Run("CannonPrestatesBaseURLTakesPrecedence", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgsExceptArr(traceType, allPrestateOptions, "--cannon-prestates-url=http://localhost/foo", "--prestates-url=http://localhost/bar"))
				require.Equal(t, "http://localhost/foo", cfg.CannonAbsolutePreStateBaseURL.String())
			})

			t.Run("RequiredIfCannonPrestatesBaseURLNotSet", func(t *testing.T) {
				verifyArgsInvalid(t, "flag prestates-url/cannon-prestates-url or cannon-prestate is required", addRequiredArgsExceptArr(traceType, allPrestateOptions))
			})

			t.Run("Invalid", func(t *testing.T) {
				verifyArgsInvalid(t, "invalid prestates-url (:foo/bar)", addRequiredArgsExceptArr(traceType, allPrestateOptions, "--prestates-url=:foo/bar"))
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgsExceptArr(traceType, allPrestateOptions, "--prestates-url=http://localhost/foo"))
				require.Equal(t, "http://localhost/foo", cfg.CannonAbsolutePreStateBaseURL.String())
			})
		})

		t.Run(fmt.Sprintf("TestL2Rpc-%v", traceType), func(t *testing.T) {
			t.Run("RequiredForCannonTrace", func(t *testing.T) {
				verifyArgsInvalid(t, "flag l2-eth-rpc is required", addRequiredArgsExcept(traceType, "--l2-eth-rpc"))
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgs(traceType))
				require.Equal(t, []string{l2EthRpc}, cfg.L2Rpcs)
			})
		})

		t.Run(fmt.Sprintf("TestCannonSnapshotFreq-%v", traceType), func(t *testing.T) {
			t.Run("UsesDefault", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgs(traceType))
				require.Equal(t, config.DefaultCannonSnapshotFreq, cfg.Cannon.SnapshotFreq)
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgs(traceType, "--cannon-snapshot-freq=1234"))
				require.Equal(t, uint(1234), cfg.Cannon.SnapshotFreq)
			})

			t.Run("Invalid", func(t *testing.T) {
				verifyArgsInvalid(t, "invalid value \"abc\" for flag -cannon-snapshot-freq",
					addRequiredArgs(traceType, "--cannon-snapshot-freq=abc"))
			})
		})

		t.Run(fmt.Sprintf("TestCannonInfoFreq-%v", traceType), func(t *testing.T) {
			t.Run("UsesDefault", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgs(traceType))
				require.Equal(t, config.DefaultCannonInfoFreq, cfg.Cannon.InfoFreq)
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgs(traceType, "--cannon-info-freq=1234"))
				require.Equal(t, uint(1234), cfg.Cannon.InfoFreq)
			})

			t.Run("Invalid", func(t *testing.T) {
				verifyArgsInvalid(t, "invalid value \"abc\" for flag -cannon-info-freq",
					addRequiredArgs(traceType, "--cannon-info-freq=abc"))
			})
		})
	}
}

func TestDepsetConfig(t *testing.T) {
	for _, traceType := range types.TraceTypes {
		if traceType == types.TraceTypeSuperCannon || traceType == types.TraceTypeSuperPermissioned {
			t.Run("Required-"+traceType.String(), func(t *testing.T) {
				verifyArgsInvalid(t,
					"flag network or rollup-config/cannon-rollup-config, l2-genesis/cannon-l2-genesis and depset-config/cannon-depset-config is required",
					addRequiredArgsExcept(traceType, "--network", "--rollup-config=rollup.json", "--l2-genesis=genesis.json"))
			})
		} else if traceType == types.TraceTypeSuperAsteriscKona {
			t.Run("Required-"+traceType.String(), func(t *testing.T) {
				verifyArgsInvalid(t,
					"flag network or rollup-config/asterisc-kona-rollup-config, l2-genesis/asterisc-kona-l2-genesis and depset-config/asterisc-kona-depset-config is required",
					addRequiredArgsExcept(traceType, "--network", "--rollup-config=rollup.json", "--l2-genesis=genesis.json"))
			})
		} else {
			t.Run("NotRequired-"+traceType.String(), func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--network", "--rollup-config=rollup.json", "--l2-genesis=genesis.json"))
				require.Equal(t, "", cfg.Cannon.DepsetConfigPath)
			})
		}
	}
}

func TestDataDir(t *testing.T) {
	for _, traceType := range types.TraceTypes {
		traceType := traceType

		t.Run(fmt.Sprintf("RequiredFor-%v", traceType), func(t *testing.T) {
			verifyArgsInvalid(t, "flag datadir is required", addRequiredArgsExcept(traceType, "--datadir"))
		})
	}

	t.Run("Valid", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgsExcept(types.TraceTypeCannon, "--datadir", "--datadir=/foo/bar/cannon"))
		require.Equal(t, "/foo/bar/cannon", cfg.Datadir)
	})
}

func TestRollupRpc(t *testing.T) {
	for _, traceType := range types.TraceTypes {
		traceType := traceType

		if traceType == types.TraceTypeSuperCannon || traceType == types.TraceTypeSuperPermissioned || traceType == types.TraceTypeSuperAsteriscKona {
			t.Run(fmt.Sprintf("NotRequiredFor-%v", traceType), func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(traceType, "--rollup-rpc"))
			})
		} else {
			t.Run(fmt.Sprintf("RequiredFor-%v", traceType), func(t *testing.T) {
				verifyArgsInvalid(t, "flag rollup-rpc is required", addRequiredArgsExcept(traceType, "--rollup-rpc"))
			})
		}
	}

	t.Run("Valid", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs(types.TraceTypeCannon))
		require.Equal(t, rollupRpc, cfg.RollupRpc)
	})
}

func TestGameWindow(t *testing.T) {
	t.Run("UsesDefault", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs(types.TraceTypeAlphabet))
		require.Equal(t, config.DefaultGameWindow, cfg.GameWindow)
	})

	t.Run("Valid", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs(types.TraceTypeAlphabet, "--game-window=1m"))
		require.Equal(t, time.Minute, cfg.GameWindow)
	})

	t.Run("ParsesDefault", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs(types.TraceTypeAlphabet, "--game-window=672h"))
		require.Equal(t, config.DefaultGameWindow, cfg.GameWindow)
	})
}

func TestUnsafeAllowInvalidPrestate(t *testing.T) {
	t.Run("DefaultsToFalse", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--unsafe-allow-invalid-prestate"))
		require.False(t, cfg.AllowInvalidPrestate)
	})

	t.Run("EnabledWithNoValue", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs(types.TraceTypeCannon, "--unsafe-allow-invalid-prestate"))
		require.True(t, cfg.AllowInvalidPrestate)
	})

	t.Run("EnabledWithTrue", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs(types.TraceTypeCannon, "--unsafe-allow-invalid-prestate=true"))
		require.True(t, cfg.AllowInvalidPrestate)
	})

	t.Run("DisabledWithFalse", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs(types.TraceTypeCannon, "--unsafe-allow-invalid-prestate=false"))
		require.False(t, cfg.AllowInvalidPrestate)
	})
}

func TestAdditionalBondClaimants(t *testing.T) {
	t.Run("DefaultsToEmpty", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgsExcept(types.TraceTypeAlphabet, "--additional-bond-claimants"))
		require.Empty(t, cfg.AdditionalBondClaimants)
	})

	t.Run("Valid-Single", func(t *testing.T) {
		claimant := common.Address{0xaa}
		cfg := configForArgs(t, addRequiredArgs(types.TraceTypeAlphabet, "--additional-bond-claimants", claimant.Hex()))
		require.Contains(t, cfg.AdditionalBondClaimants, claimant)
		require.Len(t, cfg.AdditionalBondClaimants, 1)
	})

	t.Run("Valid-Multiple", func(t *testing.T) {
		claimant1 := common.Address{0xaa}
		claimant2 := common.Address{0xbb}
		claimant3 := common.Address{0xcc}
		cfg := configForArgs(t, addRequiredArgs(types.TraceTypeAlphabet,
			"--additional-bond-claimants", fmt.Sprintf("%v,%v,%v", claimant1.Hex(), claimant2.Hex(), claimant3.Hex())))
		require.Contains(t, cfg.AdditionalBondClaimants, claimant1)
		require.Contains(t, cfg.AdditionalBondClaimants, claimant2)
		require.Contains(t, cfg.AdditionalBondClaimants, claimant3)
		require.Len(t, cfg.AdditionalBondClaimants, 3)
	})

	t.Run("Invalid-Single", func(t *testing.T) {
		verifyArgsInvalid(t, "invalid additional claimant",
			addRequiredArgs(types.TraceTypeAlphabet, "--additional-bond-claimants", "nope"))
	})

	t.Run("Invalid-Multiple", func(t *testing.T) {
		claimant1 := common.Address{0xaa}
		claimant2 := common.Address{0xbb}
		verifyArgsInvalid(t, "invalid additional claimant",
			addRequiredArgs(types.TraceTypeAlphabet, "--additional-bond-claimants", fmt.Sprintf("%v,nope,%v", claimant1.Hex(), claimant2.Hex())))
	})
}

func TestSignerTLS(t *testing.T) {
	t.Run("EnabledByDefault", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs(types.TraceTypeAlphabet))
		require.True(t, cfg.TxMgrConfig.SignerCLIConfig.TLSConfig.Enabled)
	})

	t.Run("Disabled", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs(types.TraceTypeAlphabet, "--signer.tls.enabled=false"))
		require.False(t, cfg.TxMgrConfig.SignerCLIConfig.TLSConfig.Enabled)
	})
}

func verifyArgsInvalid(t *testing.T, messageContains string, cliArgs []string) {
	_, _, err := dryRunWithArgs(cliArgs)
	require.ErrorContains(t, err, messageContains)
}

func configForArgs(t *testing.T, cliArgs []string) config.Config {
	_, cfg, err := dryRunWithArgs(cliArgs)
	require.NoError(t, err)
	return cfg
}

func dryRunWithArgs(cliArgs []string) (log.Logger, config.Config, error) {
	cfg := new(config.Config)
	var logger log.Logger
	fullArgs := append([]string{"op-challenger"}, cliArgs...)
	testErr := errors.New("dry-run")
	err := run(context.Background(), fullArgs, func(ctx context.Context, log log.Logger, config *config.Config) (cliapp.Lifecycle, error) {
		logger = log
		cfg = config
		return nil, testErr
	})
	if errors.Is(err, testErr) { // expected error
		err = nil
	}
	return logger, *cfg, err
}

func addRequiredArgs(traceType types.TraceType, args ...string) []string {
	req := requiredArgs(traceType)
	combined := toArgList(req)
	return append(combined, args...)
}

func addRequiredArgsExcept(traceType types.TraceType, name string, optionalArgs ...string) []string {
	req := requiredArgs(traceType)
	delete(req, name)
	return append(toArgList(req), optionalArgs...)
}

func addRequiredArgsForMultipleTracesExcept(traceType []types.TraceType, name string, optionalArgs ...string) []string {
	req := requiredArgsMultiple(traceType)
	delete(req, name)
	return append(toArgList(req), optionalArgs...)
}

func addRequiredArgsExceptArr(traceType types.TraceType, names []string, optionalArgs ...string) []string {
	req := requiredArgs(traceType)
	for _, name := range names {
		delete(req, name)
	}
	return append(toArgList(req), optionalArgs...)
}

func requiredArgsMultiple(traceType []types.TraceType) map[string]string {
	args := make(map[string]string)
	for _, t := range traceType {
		for name, value := range requiredArgs(t) {
			args[name] = value
		}
	}
	return args
}

func requiredArgs(traceType types.TraceType) map[string]string {
	args := map[string]string{
		"--l1-eth-rpc":           l1EthRpc,
		"--l1-beacon":            l1Beacon,
		"--l2-eth-rpc":           l2EthRpc,
		"--game-factory-address": gameFactoryAddressValue,
		"--trace-type":           traceType.String(),
		"--datadir":              datadir,
	}
	switch traceType {
	case types.TraceTypeCannon, types.TraceTypePermissioned:
		addRequiredCannonArgs(args)
	case types.TraceTypeAsterisc:
		addRequiredAsteriscArgs(args)
	case types.TraceTypeAsteriscKona:
		addRequiredAsteriscKonaArgs(args)
	case types.TraceTypeSuperCannon, types.TraceTypeSuperPermissioned:
		addRequiredSuperCannonArgs(args)
	case types.TraceTypeSuperAsteriscKona:
		addRequiredSuperAsteriscKonaArgs(args)
	case types.TraceTypeAlphabet, types.TraceTypeFast:
		addRequiredOutputRootArgs(args)
	}
	return args
}

func addRequiredSuperCannonArgs(args map[string]string) {
	addRequiredCannonBaseArgs(args)
	args["--supervisor-rpc"] = supervisorRpc
}

func addRequiredCannonArgs(args map[string]string) {
	addRequiredCannonBaseArgs(args)
	addRequiredOutputRootArgs(args)
}

func addRequiredOutputRootArgs(args map[string]string) {
	args["--rollup-rpc"] = rollupRpc
}

func addRequiredCannonBaseArgs(args map[string]string) {
	args["--network"] = network
	args["--cannon-bin"] = cannonBin
	args["--cannon-server"] = cannonServer
	args["--cannon-prestate"] = cannonPreState
}

func addRequiredAsteriscArgs(args map[string]string) {
	addRequiredOutputRootArgs(args)
	args["--network"] = network
	args["--asterisc-bin"] = asteriscBin
	args["--asterisc-server"] = asteriscServer
	args["--asterisc-prestate"] = asteriscPreState
}

func addRequiredAsteriscKonaArgs(args map[string]string) {
	addRequiredOutputRootArgs(args)
	args["--network"] = network
	args["--asterisc-bin"] = asteriscBin
	args["--asterisc-kona-server"] = asteriscServer
	args["--asterisc-kona-prestate"] = asteriscPreState
}

func addRequiredSuperAsteriscKonaArgs(args map[string]string) {
	addRequiredAsteriscKonaArgs(args)
	args["--supervisor-rpc"] = supervisorRpc
}

func toArgList(req map[string]string) []string {
	var combined []string
	for name, value := range req {
		combined = append(combined, fmt.Sprintf("%s=%s", name, value))
	}
	return combined
}
