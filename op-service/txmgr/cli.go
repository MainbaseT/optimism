package txmgr

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync/atomic"
	"time"

	opservice "github.com/ethereum-optimism/optimism/op-service"
	opcrypto "github.com/ethereum-optimism/optimism/op-service/crypto"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	opsigner "github.com/ethereum-optimism/optimism/op-service/signer"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/urfave/cli/v2"
)

const (
	// Duplicated L1 RPC flag
	L1RPCFlagName = "l1-eth-rpc"
	// Key Management Flags (also have signer client flags)
	MnemonicFlagName   = "mnemonic"
	HDPathFlagName     = "hd-path"
	PrivateKeyFlagName = "private-key"
	// TxMgr Flags (new + legacy + some shared flags)
	NumConfirmationsFlagName           = "num-confirmations"
	SafeAbortNonceTooLowCountFlagName  = "safe-abort-nonce-too-low-count"
	FeeLimitMultiplierFlagName         = "fee-limit-multiplier"
	FeeLimitThresholdFlagName          = "txmgr.fee-limit-threshold"
	MinBaseFeeFlagName                 = "txmgr.min-basefee"
	MaxBaseFeeFlagName                 = "txmgr.max-basefee"
	MinTipCapFlagName                  = "txmgr.min-tip-cap"
	MaxTipCapFlagName                  = "txmgr.max-tip-cap"
	RebroadcastIntervalFlagName        = "txmgr.rebroadcast-interval"
	ResubmissionTimeoutFlagName        = "resubmission-timeout"
	NetworkTimeoutFlagName             = "network-timeout"
	RetryIntervalFlagName              = "txmgr.retry-interval"
	MaxRetriesFlagName                 = "txmgr.max-retries"
	TxSendTimeoutFlagName              = "txmgr.send-timeout"
	TxNotInMempoolTimeoutFlagName      = "txmgr.not-in-mempool-timeout"
	ReceiptQueryIntervalFlagName       = "txmgr.receipt-query-interval"
	AlreadyPublishedCustomErrsFlagName = "txmgr.already-published-custom-errs"
)

var (
	SequencerHDPathFlag = &cli.StringFlag{
		Name: "sequencer-hd-path",
		Usage: "DEPRECATED: The HD path used to derive the sequencer wallet from the " +
			"mnemonic. The mnemonic flag must also be set.",
		EnvVars: []string{"OP_BATCHER_SEQUENCER_HD_PATH"},
	}
	L2OutputHDPathFlag = &cli.StringFlag{
		Name: "l2-output-hd-path",
		Usage: "DEPRECATED:The HD path used to derive the l2output wallet from the " +
			"mnemonic. The mnemonic flag must also be set.",
		EnvVars: []string{"OP_PROPOSER_L2_OUTPUT_HD_PATH"},
	}
)

type DefaultFlagValues struct {
	NumConfirmations          uint64
	SafeAbortNonceTooLowCount uint64
	FeeLimitMultiplier        uint64
	FeeLimitThresholdGwei     float64
	MinTipCapGwei             float64
	MinBaseFeeGwei            float64
	RebroadcastInterval       time.Duration
	ResubmissionTimeout       time.Duration
	NetworkTimeout            time.Duration
	RetryInterval             time.Duration
	MaxRetries                uint64
	TxSendTimeout             time.Duration
	TxNotInMempoolTimeout     time.Duration
	ReceiptQueryInterval      time.Duration
}

var (
	DefaultBatcherFlagValues = DefaultFlagValues{
		NumConfirmations:          uint64(10),
		SafeAbortNonceTooLowCount: uint64(3),
		FeeLimitMultiplier:        uint64(5),
		FeeLimitThresholdGwei:     100.0,
		MinTipCapGwei:             1.0,
		MinBaseFeeGwei:            1.0,
		RebroadcastInterval:       12 * time.Second,
		ResubmissionTimeout:       48 * time.Second,
		NetworkTimeout:            10 * time.Second,
		RetryInterval:             1 * time.Second,
		MaxRetries:                uint64(10),
		TxSendTimeout:             0, // Try sending txs indefinitely, to preserve tx ordering for Holocene
		TxNotInMempoolTimeout:     2 * time.Minute,
		ReceiptQueryInterval:      12 * time.Second,
	}
	DefaultChallengerFlagValues = DefaultFlagValues{
		NumConfirmations:          uint64(3),
		SafeAbortNonceTooLowCount: uint64(3),
		FeeLimitMultiplier:        uint64(5),
		FeeLimitThresholdGwei:     100.0,
		MinTipCapGwei:             1.0,
		MinBaseFeeGwei:            1.0,
		ResubmissionTimeout:       24 * time.Second,
		NetworkTimeout:            10 * time.Second,
		RetryInterval:             1 * time.Second,
		MaxRetries:                uint64(10),
		TxSendTimeout:             2 * time.Minute,
		TxNotInMempoolTimeout:     1 * time.Minute,
		ReceiptQueryInterval:      12 * time.Second,
	}

	// geth enforces a 1 gwei minimum for blob tx fee
	defaultMinBlobTxFee = big.NewInt(params.GWei)
)

func CLIFlags(envPrefix string) []cli.Flag {
	return CLIFlagsWithDefaults(envPrefix, DefaultBatcherFlagValues)
}

func CLIFlagsWithDefaults(envPrefix string, defaults DefaultFlagValues) []cli.Flag {
	prefixEnvVars := func(name string) []string {
		return opservice.PrefixEnvVar(envPrefix, name)
	}
	return append([]cli.Flag{
		&cli.StringFlag{
			Name:    MnemonicFlagName,
			Usage:   "The mnemonic used to derive the wallets for either the service",
			EnvVars: prefixEnvVars("MNEMONIC"),
		},
		&cli.StringFlag{
			Name:    HDPathFlagName,
			Usage:   "The HD path used to derive the sequencer wallet from the mnemonic. The mnemonic flag must also be set.",
			EnvVars: prefixEnvVars("HD_PATH"),
		},
		&cli.StringFlag{
			Name:    PrivateKeyFlagName,
			Usage:   "The private key to use with the service. Must not be used with mnemonic.",
			EnvVars: prefixEnvVars("PRIVATE_KEY"),
		},
		&cli.Uint64Flag{
			Name:    NumConfirmationsFlagName,
			Usage:   "Number of confirmations which we will wait after sending a transaction",
			Value:   defaults.NumConfirmations,
			EnvVars: prefixEnvVars("NUM_CONFIRMATIONS"),
		},
		&cli.Uint64Flag{
			Name:    SafeAbortNonceTooLowCountFlagName,
			Usage:   "Number of ErrNonceTooLow observations required to give up on a tx at a particular nonce without receiving confirmation",
			Value:   defaults.SafeAbortNonceTooLowCount,
			EnvVars: prefixEnvVars("SAFE_ABORT_NONCE_TOO_LOW_COUNT"),
		},
		&cli.Uint64Flag{
			Name:    FeeLimitMultiplierFlagName,
			Usage:   "The multiplier applied to fee suggestions to put a hard limit on fee increases",
			Value:   defaults.FeeLimitMultiplier,
			EnvVars: prefixEnvVars("TXMGR_FEE_LIMIT_MULTIPLIER"),
		},
		&cli.Float64Flag{
			Name:    FeeLimitThresholdFlagName,
			Usage:   "The minimum threshold (in GWei) at which fee bumping starts to be capped. Allows arbitrary fee bumps below this threshold.",
			Value:   defaults.FeeLimitThresholdGwei,
			EnvVars: prefixEnvVars("TXMGR_FEE_LIMIT_THRESHOLD"),
		},
		&cli.Float64Flag{
			Name:    MinTipCapFlagName,
			Usage:   "Enforces a minimum tip cap (in GWei) to use when determining tx fees. 1 GWei by default.",
			Value:   defaults.MinTipCapGwei,
			EnvVars: prefixEnvVars("TXMGR_MIN_TIP_CAP"),
		},
		&cli.Float64Flag{
			Name:    MaxTipCapFlagName,
			Usage:   "Enforces a maximum tip cap (in GWei) to use when determining tx fees, `TxMgr` returns an error when exceeded. Disabled by default.",
			EnvVars: prefixEnvVars("TXMGR_MAX_TIP_CAP"),
		},
		&cli.Float64Flag{
			Name:    MinBaseFeeFlagName,
			Usage:   "Enforces a minimum base fee (in GWei) to assume when determining tx fees. 1 GWei by default.",
			Value:   defaults.MinBaseFeeGwei,
			EnvVars: prefixEnvVars("TXMGR_MIN_BASEFEE"),
		},
		&cli.Float64Flag{
			Name:    MaxBaseFeeFlagName,
			Usage:   "Enforces a maximum base fee (in GWei) to assume when determining tx fees, `TxMgr` returns an error when exceeded. Disabled by default.",
			EnvVars: prefixEnvVars("TXMGR_MAX_BASEFEE"),
		},
		&cli.DurationFlag{
			Name:    RebroadcastIntervalFlagName,
			Usage:   "Interval at which a published transaction will be rebroadcasted if it has not yet been mined. Should be less than ResubmissionTimeout to have an effect.",
			Value:   defaults.RebroadcastInterval,
			EnvVars: prefixEnvVars("TXMGR_REBROADCAST_INTERVAL"),
		},
		&cli.DurationFlag{
			Name:    ResubmissionTimeoutFlagName,
			Usage:   "Duration we will wait before resubmitting a transaction to L1",
			Value:   defaults.ResubmissionTimeout,
			EnvVars: prefixEnvVars("RESUBMISSION_TIMEOUT"),
		},
		&cli.DurationFlag{
			Name:    NetworkTimeoutFlagName,
			Usage:   "Timeout for all network operations",
			Value:   defaults.NetworkTimeout,
			EnvVars: prefixEnvVars("NETWORK_TIMEOUT"),
		},
		&cli.DurationFlag{
			Name:    RetryIntervalFlagName,
			Usage:   "Duration we will wait before resubmitting a transaction to L1 on a transient error. Values <= 0 will result in retrying immediately. Should be less than ResubmissionTimeout to have an effect.",
			Value:   defaults.RetryInterval,
			EnvVars: prefixEnvVars("TXMGR_RETRY_INTERVAL"),
		},
		&cli.Uint64Flag{
			Name:    MaxRetriesFlagName,
			Usage:   "Maximum number of times to resubmit a transaction to L1 on a transient error. Set to 0 to disable retries.",
			Value:   defaults.MaxRetries,
			EnvVars: prefixEnvVars("TXMGR_MAX_RETRIES"),
		},
		&cli.DurationFlag{
			Name:    TxSendTimeoutFlagName,
			Usage:   "Timeout for sending transactions. If 0 it is disabled.",
			Value:   defaults.TxSendTimeout,
			EnvVars: prefixEnvVars("TXMGR_TX_SEND_TIMEOUT"),
		},
		&cli.DurationFlag{
			Name:    TxNotInMempoolTimeoutFlagName,
			Usage:   "Timeout for aborting a tx send if the tx does not make it to the mempool.",
			Value:   defaults.TxNotInMempoolTimeout,
			EnvVars: prefixEnvVars("TXMGR_TX_NOT_IN_MEMPOOL_TIMEOUT"),
		},
		&cli.DurationFlag{
			Name:    ReceiptQueryIntervalFlagName,
			Usage:   "Frequency to poll for receipts",
			Value:   defaults.ReceiptQueryInterval,
			EnvVars: prefixEnvVars("TXMGR_RECEIPT_QUERY_INTERVAL"),
		},
		&cli.StringSliceFlag{
			Name:    AlreadyPublishedCustomErrsFlagName,
			Usage:   "List of custom RPC error messages that indicate that a transaction has already been published.",
			EnvVars: prefixEnvVars("TXMGR_ALREADY_PUBLISHED_CUSTOM_ERRS"),
		},
	}, opsigner.CLIFlags(envPrefix, "")...)
}

type CLIConfig struct {
	L1RPCURL                   string
	Mnemonic                   string
	HDPath                     string
	SequencerHDPath            string
	L2OutputHDPath             string
	PrivateKey                 string
	SignerCLIConfig            opsigner.CLIConfig
	NumConfirmations           uint64
	SafeAbortNonceTooLowCount  uint64
	FeeLimitMultiplier         uint64
	FeeLimitThresholdGwei      float64
	MinBaseFeeGwei             float64
	MinTipCapGwei              float64
	MaxBaseFeeGwei             float64
	MaxTipCapGwei              float64
	RebroadcastInterval        time.Duration
	ResubmissionTimeout        time.Duration
	ReceiptQueryInterval       time.Duration
	NetworkTimeout             time.Duration
	RetryInterval              time.Duration
	MaxRetries                 uint64
	TxSendTimeout              time.Duration
	TxNotInMempoolTimeout      time.Duration
	AlreadyPublishedCustomErrs []string
}

func NewCLIConfig(l1RPCURL string, defaults DefaultFlagValues) CLIConfig {
	return CLIConfig{
		L1RPCURL:                  l1RPCURL,
		NumConfirmations:          defaults.NumConfirmations,
		SafeAbortNonceTooLowCount: defaults.SafeAbortNonceTooLowCount,
		FeeLimitMultiplier:        defaults.FeeLimitMultiplier,
		FeeLimitThresholdGwei:     defaults.FeeLimitThresholdGwei,
		MinTipCapGwei:             defaults.MinTipCapGwei,
		MinBaseFeeGwei:            defaults.MinBaseFeeGwei,
		RebroadcastInterval:       defaults.RebroadcastInterval,
		ResubmissionTimeout:       defaults.ResubmissionTimeout,
		NetworkTimeout:            defaults.NetworkTimeout,
		RetryInterval:             defaults.RetryInterval,
		MaxRetries:                defaults.MaxRetries,
		TxSendTimeout:             defaults.TxSendTimeout,
		TxNotInMempoolTimeout:     defaults.TxNotInMempoolTimeout,
		ReceiptQueryInterval:      defaults.ReceiptQueryInterval,
		SignerCLIConfig:           opsigner.NewCLIConfig(),
	}
}

func (m CLIConfig) Check() error {
	if m.L1RPCURL == "" {
		return errors.New("must provide a L1 RPC url")
	}
	if m.NumConfirmations == 0 {
		return errors.New("NumConfirmations must not be 0")
	}
	if m.NetworkTimeout == 0 {
		return errors.New("must provide NetworkTimeout")
	}
	if m.FeeLimitMultiplier == 0 {
		return errors.New("must provide FeeLimitMultiplier")
	}
	if m.MinBaseFeeGwei < m.MinTipCapGwei {
		return fmt.Errorf("minBaseFee smaller than minTipCap, have %f < %f",
			m.MinBaseFeeGwei, m.MinTipCapGwei)
	}
	if m.ResubmissionTimeout == 0 {
		return errors.New("must provide ResubmissionTimeout")
	}
	if m.ReceiptQueryInterval == 0 {
		return errors.New("must provide ReceiptQueryInterval")
	}
	if m.TxNotInMempoolTimeout == 0 {
		return errors.New("must provide TxNotInMempoolTimeout")
	}
	if m.SafeAbortNonceTooLowCount == 0 {
		return errors.New("SafeAbortNonceTooLowCount must not be 0")
	}
	if err := m.SignerCLIConfig.Check(); err != nil {
		return err
	}
	atMostOneIsSet := func(options ...bool) bool {
		boolToInt := func(b bool) int {
			if b {
				return 1
			}
			return 0
		}

		sum := 0
		for _, option := range options {
			sum += boolToInt(option)
		}
		return sum == 1 || sum == 0
	}
	if !atMostOneIsSet(m.PrivateKey != "", m.Mnemonic != "", m.SignerCLIConfig.Enabled()) {
		return errors.New("can only provide at most one of: [private key, mnemonic, remote signer]")
	}
	return nil
}

func ReadCLIConfig(ctx *cli.Context) CLIConfig {
	return CLIConfig{
		L1RPCURL:                   ctx.String(L1RPCFlagName),
		Mnemonic:                   ctx.String(MnemonicFlagName),
		HDPath:                     ctx.String(HDPathFlagName),
		SequencerHDPath:            ctx.String(SequencerHDPathFlag.Name),
		L2OutputHDPath:             ctx.String(L2OutputHDPathFlag.Name),
		PrivateKey:                 ctx.String(PrivateKeyFlagName),
		SignerCLIConfig:            opsigner.ReadCLIConfig(ctx),
		NumConfirmations:           ctx.Uint64(NumConfirmationsFlagName),
		SafeAbortNonceTooLowCount:  ctx.Uint64(SafeAbortNonceTooLowCountFlagName),
		FeeLimitMultiplier:         ctx.Uint64(FeeLimitMultiplierFlagName),
		FeeLimitThresholdGwei:      ctx.Float64(FeeLimitThresholdFlagName),
		MinBaseFeeGwei:             ctx.Float64(MinBaseFeeFlagName),
		MaxBaseFeeGwei:             ctx.Float64(MaxBaseFeeFlagName),
		MinTipCapGwei:              ctx.Float64(MinTipCapFlagName),
		MaxTipCapGwei:              ctx.Float64(MaxTipCapFlagName),
		RebroadcastInterval:        ctx.Duration(RebroadcastIntervalFlagName),
		ResubmissionTimeout:        ctx.Duration(ResubmissionTimeoutFlagName),
		ReceiptQueryInterval:       ctx.Duration(ReceiptQueryIntervalFlagName),
		NetworkTimeout:             ctx.Duration(NetworkTimeoutFlagName),
		RetryInterval:              ctx.Duration(RetryIntervalFlagName),
		MaxRetries:                 ctx.Uint64(MaxRetriesFlagName),
		TxSendTimeout:              ctx.Duration(TxSendTimeoutFlagName),
		TxNotInMempoolTimeout:      ctx.Duration(TxNotInMempoolTimeoutFlagName),
		AlreadyPublishedCustomErrs: ctx.StringSlice(AlreadyPublishedCustomErrsFlagName),
	}
}

func NewConfig(cfg CLIConfig, l log.Logger) (*Config, error) {
	if err := cfg.Check(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.NetworkTimeout)
	defer cancel()
	l1, err := ethclient.DialContext(ctx, cfg.L1RPCURL)
	if err != nil {
		return nil, fmt.Errorf("could not dial eth client: %w", err)
	}

	ctx, cancel = context.WithTimeout(context.Background(), cfg.NetworkTimeout)
	defer cancel()
	chainID, err := l1.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not dial fetch L1 chain ID: %w", err)
	}

	// Allow backwards compatible ways of specifying the HD path
	hdPath := cfg.HDPath
	if hdPath == "" && cfg.SequencerHDPath != "" {
		hdPath = cfg.SequencerHDPath
	} else if hdPath == "" && cfg.L2OutputHDPath != "" {
		hdPath = cfg.L2OutputHDPath
	}

	signerFactory, from, err := opcrypto.SignerFactoryFromConfig(l, cfg.PrivateKey, cfg.Mnemonic, hdPath, cfg.SignerCLIConfig)
	if err != nil {
		return nil, fmt.Errorf("could not init signer: %w", err)
	}

	feeLimitThreshold, err := eth.GweiToWei(cfg.FeeLimitThresholdGwei)
	if err != nil {
		return nil, fmt.Errorf("invalid fee limit threshold: %w", err)
	}

	minBaseFee, err := eth.GweiToWei(cfg.MinBaseFeeGwei)
	if err != nil {
		return nil, fmt.Errorf("invalid min base fee: %w", err)
	}

	minTipCap, err := eth.GweiToWei(cfg.MinTipCapGwei)
	if err != nil {
		return nil, fmt.Errorf("invalid min tip cap: %w", err)
	}

	var (
		maxBaseFee, maxTipCap *big.Int
	)
	if cfg.MaxBaseFeeGwei > 0 {
		maxBaseFee, err = eth.GweiToWei(cfg.MaxBaseFeeGwei)
		if err != nil {
			return nil, fmt.Errorf("invalid max base fee: %w", err)
		}
	}

	if cfg.MaxTipCapGwei > 0 {
		maxTipCap, err = eth.GweiToWei(cfg.MaxTipCapGwei)
		if err != nil {
			return nil, fmt.Errorf("invalid max tip cap: %w", err)
		}
	}

	res := Config{
		Backend: l1,
		ChainID: chainID,
		Signer:  signerFactory(chainID),
		From:    from,

		TxSendTimeout:              cfg.TxSendTimeout,
		TxNotInMempoolTimeout:      cfg.TxNotInMempoolTimeout,
		NetworkTimeout:             cfg.NetworkTimeout,
		RetryInterval:              cfg.RetryInterval,
		MaxRetries:                 cfg.MaxRetries,
		ReceiptQueryInterval:       cfg.ReceiptQueryInterval,
		NumConfirmations:           cfg.NumConfirmations,
		SafeAbortNonceTooLowCount:  cfg.SafeAbortNonceTooLowCount,
		AlreadyPublishedCustomErrs: cfg.AlreadyPublishedCustomErrs,
	}

	res.RebroadcastInterval.Store(int64(cfg.RebroadcastInterval))
	res.ResubmissionTimeout.Store(int64(cfg.ResubmissionTimeout))
	res.FeeLimitThreshold.Store(feeLimitThreshold)
	res.FeeLimitMultiplier.Store(cfg.FeeLimitMultiplier)
	res.MinBaseFee.Store(minBaseFee)
	res.MaxBaseFee.Store(maxBaseFee)
	res.MinTipCap.Store(minTipCap)
	res.MaxTipCap.Store(maxTipCap)
	res.MinBlobTxFee.Store(defaultMinBlobTxFee)

	return &res, nil
}

// Config houses parameters for altering the behavior of a SimpleTxManager.
type Config struct {
	Backend ETHBackend

	// RebroadcastInterval is the interval at which a published transaction
	// will be rebroadcasted if it has not yet been mined.
	RebroadcastInterval atomic.Int64

	// ResubmissionTimeout is the interval at which, if no previously
	// published transaction has been mined, the new tx with a bumped gas
	// price will be published. Only one publication at MaxGasPrice will be
	// attempted.
	ResubmissionTimeout atomic.Int64

	// The multiplier applied to fee suggestions to put a hard limit on fee increases.
	FeeLimitMultiplier atomic.Uint64

	// Minimum threshold (in Wei) at which the FeeLimitMultiplier takes effect.
	// On low-fee networks, like test networks, this allows for arbitrary fee bumps
	// below this threshold.
	FeeLimitThreshold atomic.Pointer[big.Int]

	// Minimum base fee (in Wei) to assume when determining tx fees.
	MinBaseFee atomic.Pointer[big.Int]
	// Maximum base fee (in Wei) to assume when determining tx fees.
	MaxBaseFee atomic.Pointer[big.Int]

	// Minimum tip cap (in Wei) to enforce when determining tx fees.
	MinTipCap atomic.Pointer[big.Int]
	// Maximum tip cap (in Wei) to enforce when determining tx fees.
	MaxTipCap atomic.Pointer[big.Int]

	MinBlobTxFee atomic.Pointer[big.Int]

	// ChainID is the chain ID of the L1 chain.
	ChainID *big.Int

	// TxSendTimeout is how long to wait for sending a transaction.
	// By default it is unbounded. If set, this is recommended to be at least 20 minutes.
	TxSendTimeout time.Duration

	// TxNotInMempoolTimeout is how long to wait before aborting a transaction send if the transaction does not
	// make it to the mempool. If the tx is in the mempool, TxSendTimeout is used instead.
	TxNotInMempoolTimeout time.Duration

	// NetworkTimeout is the allowed duration for a single network request.
	// This is intended to be used for network requests that can be replayed.
	NetworkTimeout time.Duration

	// RetryInterval is the interval at which the tx manager will retry
	// sending a transaction if it fails with a non-fatal error (e.g. the
	// gapped nonce error in the blob pool).
	RetryInterval time.Duration

	// MaxRetries is the maximum number of times to retry sending a
	// transaction. This is used to limit the number of times we retry
	// sending a transaction if it fails with a non-fatal error (e.g. the
	// gapped nonce error in the blob pool).
	MaxRetries uint64

	// ReceiptQueryInterval is the interval at which the tx manager will
	// query the backend to check for confirmations after a tx at a
	// specific gas price has been published.
	ReceiptQueryInterval time.Duration

	// NumConfirmations specifies how many blocks are need to consider a
	// transaction confirmed.
	NumConfirmations uint64

	// SafeAbortNonceTooLowCount specifies how many ErrNonceTooLow observations
	// are required to give up on a tx at a particular nonce without receiving
	// confirmation.
	SafeAbortNonceTooLowCount uint64

	// Signer is used to sign transactions when the gas price is increased.
	Signer opcrypto.SignerFn
	From   common.Address

	// GasPriceEstimatorFn is used to estimate the gas price for a transaction.
	// If nil, DefaultGasPriceEstimatorFn is used.
	GasPriceEstimatorFn GasPriceEstimatorFn

	// List of custom RPC error messages that indicate that a transaction has
	// already been published.
	AlreadyPublishedCustomErrs []string
}

func (m *Config) Check() error {
	if m.Backend == nil {
		return errors.New("must provide the Backend")
	}
	if m.NumConfirmations == 0 {
		return errors.New("NumConfirmations must not be 0")
	}
	if m.NetworkTimeout == 0 {
		return errors.New("must provide NetworkTimeout")
	}
	if m.FeeLimitMultiplier.Load() == 0 {
		return errors.New("must provide FeeLimitMultiplier")
	}
	minBaseFee := m.MinBaseFee.Load()
	minTipCap := m.MinTipCap.Load()
	if minBaseFee != nil && minTipCap != nil && minBaseFee.Cmp(minTipCap) == -1 {
		return fmt.Errorf("minBaseFee smaller than minTipCap, have %v < %v",
			minBaseFee, minTipCap)
	}
	if m.ResubmissionTimeout.Load() == 0 {
		return errors.New("must provide ResubmissionTimeout")
	}
	if m.ReceiptQueryInterval == 0 {
		return errors.New("must provide ReceiptQueryInterval")
	}
	if m.TxNotInMempoolTimeout == 0 {
		return errors.New("must provide TxNotInMempoolTimeout")
	}
	if m.SafeAbortNonceTooLowCount == 0 {
		return errors.New("SafeAbortNonceTooLowCount must not be 0")
	}
	if m.Signer == nil {
		return errors.New("must provide the Signer")
	}
	if m.ChainID == nil {
		return errors.New("must provide the ChainID")
	}
	return nil
}
