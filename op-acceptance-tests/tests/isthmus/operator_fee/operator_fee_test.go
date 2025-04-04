package operatorfee

import (
	"encoding/hex"
	"log/slog"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/systest"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/testlib/validators"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"
)

// TestFees verifies that L1/L2 fees are handled properly in different fork configurations
func TestOperatorFee(t *testing.T) {
	logger := testlog.Logger(t, slog.LevelDebug)
	// Define which L2 chain we'll test
	chainIdx := uint64(0)

	logger.Info("Starting operator fee test", "chain", chainIdx)

	// Get validators and getters for accessing the system and wallets
	l1WalletGetter, l1WalletValidator := validators.AcquireL1WalletWithFunds(types.NewBalance(big.NewInt(params.Ether)))
	l2WalletGetter, l2WalletValidator := validators.AcquireL2WalletWithFunds(chainIdx, types.NewBalance(big.NewInt(params.Ether)))

	logger.Info("Acquired test wallets with funds")

	// Run isthmus test
	_, forkValidator := validators.AcquireL2WithFork(chainIdx, rollup.Isthmus)
	nodesValidator := validators.HasSufficientL2Nodes(chainIdx, 2)
	logger.Info("Running system test", "fork", "Isthmus", "nodes", 2)
	systest.SystemTest(t,
		func(t systest.T, sys system.System) {
			logger.Info("Starting operator fee test scenario", "chain", chainIdx)

			l1Wallet, err := system.NewWalletV2FromWalletAndChain(t.Context(), l1WalletGetter(t.Context()), sys.L1())
			require.NoError(t, err)

			l2Wallet, err := system.NewWalletV2FromWalletAndChain(t.Context(), l2WalletGetter(t.Context()), sys.L2s()[0])
			require.NoError(t, err)

			// Define test cases with different operator fee parameters
			numRandomValuesForEachDimm := 1
			testCases := GenerateAllTestParamsCases(numRandomValuesForEachDimm)

			// For each test case, verify the operator fee parameters
			for _, tc := range testCases {
				t.Run(tc.ID, func(t systest.T) {
					operatorFeeTestProcedure(t, sys, l1Wallet, l2Wallet, chainIdx, tc, logger)
				})
			}
		},
		l2WalletValidator,
		l1WalletValidator,
		forkValidator,
		nodesValidator,
	)
}

func operatorFeeTestProcedure(t systest.T, sys system.System, l1FundingWallet system.WalletV2, l2FundingWallet system.WalletV2, chainIdx uint64, tc TestParams, logger log.Logger) {
	ctx := t.Context()
	logger.Info("Starting operator fee test",
		"test_case", tc.ID,
		"operator_fee_constant", tc.OperatorFeeConstant,
		"operator_fee_scalar", tc.OperatorFeeScalar,
		"l1_fee_constant", tc.L1BlobBaseFeeScalar,
		"l1_fee_scalar", tc.L1BaseFeeScalar,
	)

	// ==========
	// Read-only Test Setup + Invariant Checks
	// ==========

	// Setup clients
	logger.Info("Setting up clients for L1 and L2 chains")
	l1GethClient, err := sys.L1().Nodes()[0].GethClient()
	require.NoError(t, err)
	l2Chain := sys.L2s()[chainIdx]
	l2GethSeqClient, err := l2Chain.Nodes()[0].GethClient()
	require.NoError(t, err)

	// Setup chain fork detection
	secondCheck, err := systest.CheckForChainFork(t.Context(), l2Chain, logger)
	require.NoError(t, err, "error checking for chain fork")
	defer func() {
		require.NoError(t, secondCheck(t.Failed()), "error checking for chain fork")
	}()

	l2StartHeader, err := l2GethSeqClient.HeaderByNumber(ctx, nil)
	require.NoError(t, err)

	// Verify coinbase address is the same as the sequencer fee vault address
	require.Equal(t, l2StartHeader.Coinbase, predeploys.SequencerFeeVaultAddr, "coinbase address should always be the same as the sequencer fee vault address")

	// setup rollup owner wallet
	logger.Info("Setting up rollup owner wallet")
	l1RollupOwnerWallet_v1, ok := sys.L2s()[chainIdx].L1Wallets()["systemConfigOwner"]
	require.True(t, ok, "rollup owner wallet not found")
	l1RollupOwnerWallet, err := system.NewWalletV2FromWalletAndChain(t.Context(), l1RollupOwnerWallet_v1, sys.L1())
	require.NoError(t, err)

	l1ChainID, err := l1GethClient.ChainID(ctx)
	require.NoError(t, err)
	logger.Debug("L1 chain ID", "chainID", l1ChainID)

	// Get the genesis config
	logger.Info("Getting L2 chain config")
	l2ChainConfig, err := l2Chain.Config()
	require.NoError(t, err)

	// Create fee checker
	logger.Info("Creating fee checker utility")
	feeChecker := NewFeeChecker(t, l2GethSeqClient, l2ChainConfig, logger)

	// Setup L2 L1Block contract binding
	l2L1BlockContract, err := bindings.NewL1Block(predeploys.L1BlockAddr, l2GethSeqClient)
	require.NoError(t, err)

	// Initialize systemconfig contract
	logger.Info("Getting SystemConfig contract")
	systemConfigProxyAddr, ok := l2Chain.L1Addresses()["systemConfigProxy"]
	require.True(t, ok, "system config proxy address not found")
	systemConfig, err := bindings.NewSystemConfig(systemConfigProxyAddr, l1GethClient)
	require.NoError(t, err)

	// Verify system config proxy owner is the rollup owner
	owner, err := systemConfig.Owner(&bind.CallOpts{BlockNumber: nil})
	require.NoError(t, err)
	require.Equal(t, owner, l1RollupOwnerWallet.Address(), "system config proxy owner should be the rollup owner")

	// Create balance reader
	logger.Info("Creating balance reader")
	balanceReader := NewBalanceReader(t, l2GethSeqClient, logger)

	// Wait for first block after genesis. The genesis block has zero L1Block
	// values and will throw off the GPO checks
	logger.Info("Waiting for L2 chain to produce block 1")
	_, err = l2GethSeqClient.HeaderByNumber(ctx, big.NewInt(1))
	require.NoError(t, err)

	// Create test wallets
	logger.Info("Creating test wallet 1")
	l2TestWallet1_v1, err := NewTestWallet(ctx, l2Chain)
	require.NoError(t, err)
	l2TestWallet1, err := system.NewWalletV2FromWalletAndChain(t.Context(), l2TestWallet1_v1, l2Chain)
	require.NoError(t, err)
	logger.Info("Test wallet 1", "address", l2TestWallet1.Address().Hex(), "private key", hex.EncodeToString(l2TestWallet1.PrivateKey().D.Bytes()))

	logger.Info("Creating test wallet 2")
	l2TestWallet2_v1, err := NewTestWallet(ctx, l2Chain)
	require.NoError(t, err)
	l2TestWallet2, err := system.NewWalletV2FromWalletAndChain(t.Context(), l2TestWallet2_v1, l2Chain)
	require.NoError(t, err)
	logger.Info("Test wallet 2", "address", l2TestWallet2.Address().Hex(), "private key", hex.EncodeToString(l2TestWallet2.PrivateKey().D.Bytes()))

	fundAmount := new(big.Int).Mul(big.NewInt(1), big.NewInt(params.Ether))

	// ==========
	// Begin Test
	// ==========

	_, _, err = SendValueTx(l1FundingWallet, l1RollupOwnerWallet.Address(), new(big.Int).Mul(big.NewInt(params.Ether), big.NewInt(1)))
	require.NoError(t, err, "Error funding owner wallet")
	defer func() {
		logger.Info("Returning remaining funds to owner wallet")
		_, err := ReturnRemainingFunds(l1RollupOwnerWallet, l1FundingWallet.Address())
		require.NoError(t, err)
	}()

	// Fund test wallet from faucet
	logger.Info("Funding test wallet 1 with ETH", "amount", fundAmount)
	_, _, err = SendValueTx(l2FundingWallet, l2TestWallet1.Address(), fundAmount)
	require.NoError(t, err, "Error funding test wallet 1")
	defer func() {
		logger.Info("Returning remaining funds to test wallet 1")
		_, err := ReturnRemainingFunds(l2TestWallet1, l2FundingWallet.Address())
		require.NoError(t, err)
	}()

	// Update operator fee parameters
	logger.Info("Updating operator fee parameters",
		"constant", tc.OperatorFeeConstant,
		"scalar", tc.OperatorFeeScalar)
	receipt, err := UpdateOperatorFeeParams(systemConfig, systemConfigProxyAddr, l1RollupOwnerWallet, tc.OperatorFeeConstant, tc.OperatorFeeScalar)
	require.NoError(t, err)
	logger.Info("Operator fee parameters updated", "block", receipt.BlockNumber)

	// Update L1 fee parameters
	logger.Info("Updating L1 fee parameters",
		"l1BaseFeeScalar", tc.L1BaseFeeScalar,
		"l1BlobBaseFeeScalar", tc.L1BlobBaseFeeScalar)
	receipt, err = UpdateL1FeeParams(systemConfig, systemConfigProxyAddr, l1RollupOwnerWallet, tc.L1BaseFeeScalar, tc.L1BlobBaseFeeScalar)
	require.NoError(t, err)
	logger.Info("Operator fee parameters updated", "block", receipt.BlockNumber)

	// sleep to allow for the L2 nodes to sync to L1 origin where operator fee was set
	delay := 2 * time.Minute
	logger.Info("Waiting for L2 nodes to sync with L1 origin where operator fee was set", "delay", delay)
	time.Sleep(delay)

	// Verify L1Block contract values have been updated to match test case values
	baseFeeScalar, err := l2L1BlockContract.BaseFeeScalar(&bind.CallOpts{BlockNumber: nil})
	require.NoError(t, err)
	logger.Info("L1Block base fee scalar", "scalar", baseFeeScalar)
	require.Equal(t, tc.L1BaseFeeScalar, baseFeeScalar, "L1Block base fee scalar does not match test case value")

	blobBaseFeeScalar, err := l2L1BlockContract.BlobBaseFeeScalar(&bind.CallOpts{BlockNumber: nil})
	require.NoError(t, err)
	logger.Info("L1Block blob base fee scalar", "scalar", blobBaseFeeScalar)
	require.Equal(t, tc.L1BlobBaseFeeScalar, blobBaseFeeScalar, "L1Block blob base fee scalar does not match test case value")

	operatorFeeConstant, err := l2L1BlockContract.OperatorFeeConstant(&bind.CallOpts{BlockNumber: nil})
	require.NoError(t, err)
	logger.Info("L1Block operator fee constant", "constant", operatorFeeConstant)
	require.Equal(t, tc.OperatorFeeConstant, operatorFeeConstant, "L1Block operator fee constant does not match test case value")

	operatorFeeScalar, err := l2L1BlockContract.OperatorFeeScalar(&bind.CallOpts{BlockNumber: nil})
	require.NoError(t, err)
	logger.Info("L1Block operator fee scalar", "scalar", operatorFeeScalar)
	require.Equal(t, tc.OperatorFeeScalar, operatorFeeScalar, "L1Block operator fee scalar does not match test case value")

	l2PreTestHeader, err := l2GethSeqClient.HeaderByNumber(ctx, nil)
	require.NoError(t, err)

	// Get initial balances
	logger.Info("Sampling initial balances", "block", l2PreTestHeader.Number.Uint64())
	startBalances := balanceReader.SampleBalances(ctx, l2PreTestHeader.Number, l2TestWallet1.Address())
	logger.Debug("Initial balances", "balances", startBalances)

	// Send the test transaction
	logger.Info("Current base fee", "fee", l2PreTestHeader.BaseFee)
	tx, receipt, err := SendValueTx(l2TestWallet1, l2TestWallet2.Address(), big.NewInt(1000))
	require.NoError(t, err, "failed to send test transaction where it should succeed")

	defer func() {
		logger.Info("Returning remaining funds to test wallet 2")
		_, err := ReturnRemainingFunds(l2TestWallet2, l2FundingWallet.Address())
		require.NoError(t, err)
	}()

	// Get final balances after transaction
	logger.Info("Sampling final balances", "block", receipt.BlockNumber.Uint64())
	endBalances := balanceReader.SampleBalances(ctx, receipt.BlockNumber, l2TestWallet1.Address())
	logger.Debug("Final balances", "balances", endBalances)

	l2EndHeader, err := l2GethSeqClient.HeaderByNumber(ctx, receipt.BlockNumber)
	require.NoError(t, err)

	// Calculate expected fee changes from raw inputs
	logger.Info("Calculating expected balance changes based on transaction data")
	expectedChanges := feeChecker.CalculateExpectedBalanceChanges(
		receipt.GasUsed,
		l2EndHeader,
		tx,
	)
	logger.Debug("Expected balance changes", "changes", expectedChanges)

	// Calculate expected end balances using the new method
	expectedEndBalances := startBalances.Add(expectedChanges)
	expectedEndBalances.BlockNumber = l2EndHeader.Number
	logger.Debug("Expected final balances", "balances", expectedEndBalances)

	// Assert that actual end balances match what we calculated
	logger.Info("Verifying actual balances match expected balances")
	AssertSnapshotsEqual(t, expectedEndBalances, endBalances)
}
