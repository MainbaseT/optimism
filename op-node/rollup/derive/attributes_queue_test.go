package derive

import (
	"context"
	"math/big"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
)

// TestAttributesQueue checks that it properly uses the PreparePayloadAttributes function
// (which is well tested) and that it properly sets NoTxPool and adds in the candidate
// transactions.
func TestAttributesQueue(t *testing.T) {
	// test config, only init the necessary fields
	cfg := &rollup.Config{
		BlockTime:              2,
		L1ChainID:              big.NewInt(101),
		L2ChainID:              big.NewInt(102),
		DepositContractAddress: common.Address{0xbb},
		L1SystemConfigAddress:  common.Address{0xcc},
	}
	rng := rand.New(rand.NewSource(1234))
	l1Info := testutils.RandomBlockInfo(rng)

	l1Fetcher := &testutils.MockL1Source{}
	defer l1Fetcher.AssertExpectations(t)

	l1Fetcher.ExpectInfoByHash(l1Info.InfoHash, l1Info, nil)

	safeHead := testutils.RandomL2BlockRef(rng)
	safeHead.L1Origin = l1Info.ID()
	safeHead.Time = l1Info.InfoTime

	batch := SingularBatch{
		ParentHash:   safeHead.Hash,
		EpochNum:     rollup.Epoch(l1Info.InfoNum),
		EpochHash:    l1Info.InfoHash,
		Timestamp:    safeHead.Time + cfg.BlockTime,
		Transactions: []eth.Data{eth.Data("foobar"), eth.Data("example")},
	}

	parentL1Cfg := eth.SystemConfig{
		BatcherAddr: common.Address{42},
		Overhead:    [32]byte{},
		Scalar:      [32]byte{},
		GasLimit:    1234,
	}
	expectedL1Cfg := eth.SystemConfig{
		BatcherAddr: common.Address{42},
		Overhead:    [32]byte{},
		Scalar:      [32]byte{},
		GasLimit:    1234,
	}

	l2Fetcher := &testutils.MockL2Client{}
	l2Fetcher.ExpectSystemConfigByL2Hash(safeHead.Hash, parentL1Cfg, nil)

	rollupCfg := rollup.Config{}
	l1InfoTx, err := L1InfoDepositBytes(&rollupCfg, expectedL1Cfg, safeHead.SequenceNumber+1, l1Info, 0)
	require.NoError(t, err)
	attrs := eth.PayloadAttributes{
		Timestamp:             eth.Uint64Quantity(safeHead.Time + cfg.BlockTime),
		PrevRandao:            eth.Bytes32(l1Info.InfoMixDigest),
		SuggestedFeeRecipient: predeploys.SequencerFeeVaultAddr,
		Transactions:          []eth.Data{l1InfoTx, eth.Data("foobar"), eth.Data("example")},
		NoTxPool:              true,
		GasLimit:              (*eth.Uint64Quantity)(&expectedL1Cfg.GasLimit),
	}
	attrBuilder := NewFetchingAttributesBuilder(cfg, nil, l1Fetcher, l2Fetcher)

	aq := NewAttributesQueue(testlog.Logger(t, log.LevelError), cfg, attrBuilder, nil)

	actual, err := aq.createNextAttributes(context.Background(), &batch, safeHead)

	require.NoError(t, err)
	require.Equal(t, attrs, *actual)
}
