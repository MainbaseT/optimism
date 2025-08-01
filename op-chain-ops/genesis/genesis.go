package genesis

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus/misc/eip1559"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

// defaultGasLimit represents the default gas limit for a genesis block.
const defaultGasLimit = 30_000_000

// HoloceneExtraData represents the default extra data for Holocene-genesis chains.
var HoloceneExtraData = eip1559.EncodeHoloceneExtraData(250, 6)

// NewL2Genesis will create a new L2 genesis
func NewL2Genesis(config *DeployConfig, l1StartHeader *eth.BlockRef) (*core.Genesis, error) {
	if config.L2ChainID == 0 {
		return nil, errors.New("must define L2 ChainID")
	}

	eip1559Denom := config.EIP1559Denominator
	if eip1559Denom == 0 {
		eip1559Denom = 50
	}
	eip1559DenomCanyon := config.EIP1559DenominatorCanyon
	if eip1559DenomCanyon == 0 {
		eip1559DenomCanyon = 250
	}
	eip1559Elasticity := config.EIP1559Elasticity
	if eip1559Elasticity == 0 {
		eip1559Elasticity = 10
	}

	l1StartTime := l1StartHeader.Time

	optimismChainConfig := params.ChainConfig{
		ChainID:                 new(big.Int).SetUint64(config.L2ChainID),
		HomesteadBlock:          big.NewInt(0),
		DAOForkBlock:            nil,
		DAOForkSupport:          false,
		EIP150Block:             big.NewInt(0),
		EIP155Block:             big.NewInt(0),
		EIP158Block:             big.NewInt(0),
		ByzantiumBlock:          big.NewInt(0),
		ConstantinopleBlock:     big.NewInt(0),
		PetersburgBlock:         big.NewInt(0),
		IstanbulBlock:           big.NewInt(0),
		MuirGlacierBlock:        big.NewInt(0),
		BerlinBlock:             big.NewInt(0),
		LondonBlock:             big.NewInt(0),
		ArrowGlacierBlock:       big.NewInt(0),
		GrayGlacierBlock:        big.NewInt(0),
		MergeNetsplitBlock:      big.NewInt(0),
		TerminalTotalDifficulty: big.NewInt(0),
		BedrockBlock:            new(big.Int).SetUint64(uint64(config.L2GenesisBlockNumber)),
		RegolithTime:            config.RegolithTime(l1StartTime),
		CanyonTime:              config.CanyonTime(l1StartTime),
		ShanghaiTime:            config.CanyonTime(l1StartTime),
		CancunTime:              config.EcotoneTime(l1StartTime),
		EcotoneTime:             config.EcotoneTime(l1StartTime),
		FjordTime:               config.FjordTime(l1StartTime),
		GraniteTime:             config.GraniteTime(l1StartTime),
		HoloceneTime:            config.HoloceneTime(l1StartTime),
		IsthmusTime:             config.IsthmusTime(l1StartTime),
		JovianTime:              config.JovianTime(l1StartTime),
		PragueTime:              config.IsthmusTime(l1StartTime),
		InteropTime:             config.InteropTime(l1StartTime),
		Optimism: &params.OptimismConfig{
			EIP1559Denominator:       eip1559Denom,
			EIP1559Elasticity:        eip1559Elasticity,
			EIP1559DenominatorCanyon: &eip1559DenomCanyon,
		},
	}

	gasLimit := config.L2GenesisBlockGasLimit
	if gasLimit == 0 {
		gasLimit = defaultGasLimit
	}
	baseFee := config.L2GenesisBlockBaseFeePerGas
	if baseFee == nil {
		baseFee = newHexBig(params.InitialBaseFee)
	}
	difficulty := config.L2GenesisBlockDifficulty
	if difficulty == nil {
		difficulty = newHexBig(0)
	}

	genesis := &core.Genesis{
		Config:     &optimismChainConfig,
		Nonce:      uint64(config.L2GenesisBlockNonce),
		Timestamp:  l1StartTime,
		GasLimit:   uint64(gasLimit),
		Difficulty: difficulty.ToInt(),
		Mixhash:    config.L2GenesisBlockMixHash,
		Coinbase:   predeploys.SequencerFeeVaultAddr,
		Number:     uint64(config.L2GenesisBlockNumber),
		GasUsed:    uint64(config.L2GenesisBlockGasUsed),
		ParentHash: config.L2GenesisBlockParentHash,
		BaseFee:    baseFee.ToInt(),
		Alloc:      map[common.Address]types.Account{},
	}

	if optimismChainConfig.IsEcotone(genesis.Timestamp) {
		genesis.BlobGasUsed = u64ptr(0)
		genesis.ExcessBlobGas = u64ptr(0)
	}
	if optimismChainConfig.IsHolocene(genesis.Timestamp) {
		genesis.ExtraData = HoloceneExtraData
	}
	if optimismChainConfig.IsIsthmus(genesis.Timestamp) {
		genesis.Alloc[params.HistoryStorageAddress] = types.Account{Nonce: 1, Code: params.HistoryStorageCode, Balance: common.Big0}
	}

	return genesis, nil
}

// NewL1Genesis will create a new L1 genesis config (without the allocs part)
func NewL1Genesis(config *DeployConfig) (*core.Genesis, error) {
	if config.L1CancunTimeOffset == nil || *config.L1CancunTimeOffset != 0 {
		return nil, fmt.Errorf("expected non-nil 0 L1 cancun time offset, but got %v", config.L1CancunTimeOffset)
	}
	return NewL1GenesisMinimal(&DevL1DeployConfigMinimal{
		DevL1DeployConfig:  config.DevL1DeployConfig,
		L1ChainID:          eth.ChainIDFromUInt64(config.L1ChainID),
		L1PragueTimeOffset: (*uint64)(config.L1PragueTimeOffset),
	})
}

// DevL1DeployConfigMinimal is the minimal subset to actually create a L1 dev genesis.
type DevL1DeployConfigMinimal struct {
	DevL1DeployConfig
	L1ChainID eth.ChainID
	// When Prague activates. Relative to L1 genesis.
	L1PragueTimeOffset *uint64
}

// NewL1GenesisMinimal creates a L1 dev genesis template.
// Warning: the allocs are not included yet.
func NewL1GenesisMinimal(config *DevL1DeployConfigMinimal) (*core.Genesis, error) {
	if config.L1ChainID == eth.ChainIDFromUInt64(0) {
		return nil, errors.New("must define L1 ChainID")
	}

	chainConfig := params.ChainConfig{
		ChainID:             config.L1ChainID.ToBig(),
		HomesteadBlock:      big.NewInt(0),
		DAOForkBlock:        nil,
		DAOForkSupport:      false,
		EIP150Block:         big.NewInt(0),
		EIP155Block:         big.NewInt(0),
		EIP158Block:         big.NewInt(0),
		ByzantiumBlock:      big.NewInt(0),
		ConstantinopleBlock: big.NewInt(0),
		PetersburgBlock:     big.NewInt(0),
		IstanbulBlock:       big.NewInt(0),
		MuirGlacierBlock:    big.NewInt(0),
		BerlinBlock:         big.NewInt(0),
		LondonBlock:         big.NewInt(0),
		ArrowGlacierBlock:   big.NewInt(0),
		GrayGlacierBlock:    big.NewInt(0),
		ShanghaiTime:        u64ptr(0),
		CancunTime:          u64ptr(0),
		// To enable post-Merge consensus at genesis
		MergeNetsplitBlock:      big.NewInt(0),
		TerminalTotalDifficulty: big.NewInt(0),
		// use default Ethereum prod blob schedules
		BlobScheduleConfig: params.DefaultBlobSchedule,
	}

	gasLimit := config.L1GenesisBlockGasLimit
	if gasLimit == 0 {
		gasLimit = defaultGasLimit
	}
	baseFee := config.L1GenesisBlockBaseFeePerGas
	if baseFee == nil {
		baseFee = newHexBig(params.InitialBaseFee)
	}
	difficulty := config.L1GenesisBlockDifficulty
	if difficulty == nil {
		difficulty = newHexBig(0) // default to Merge-compatible difficulty value
	}
	timestamp := config.L1GenesisBlockTimestamp
	if timestamp == 0 {
		timestamp = hexutil.Uint64(time.Now().Unix())
	}
	if config.L1PragueTimeOffset != nil {
		pragueTime := uint64(timestamp) + uint64(*config.L1PragueTimeOffset)
		chainConfig.PragueTime = &pragueTime
	}
	// Note: excess-blob-gas, blob-gas-used, withdrawals-hash, requests-hash are set to reasonable defaults for L1 by the ToBlock() function
	return &core.Genesis{
		Config:        &chainConfig,
		Nonce:         uint64(config.L1GenesisBlockNonce),
		Timestamp:     uint64(timestamp),
		ExtraData:     make([]byte, 0),
		GasLimit:      uint64(gasLimit),
		Difficulty:    difficulty.ToInt(),
		Mixhash:       config.L1GenesisBlockMixHash,
		Coinbase:      config.L1GenesisBlockCoinbase,
		Number:        uint64(config.L1GenesisBlockNumber),
		GasUsed:       uint64(config.L1GenesisBlockGasUsed),
		ParentHash:    config.L1GenesisBlockParentHash,
		BaseFee:       baseFee.ToInt(),
		ExcessBlobGas: (*uint64)(config.L1GenesisBlockExcessBlobGas),
		BlobGasUsed:   (*uint64)(config.L1GenesisBlockBlobGasUsed),
	}, nil
}

func u64ptr(n uint64) *uint64 {
	return &n
}
