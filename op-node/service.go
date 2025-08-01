package opnode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	altda "github.com/ethereum-optimism/optimism/op-alt-da"
	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	"github.com/ethereum-optimism/optimism/op-node/config"
	"github.com/ethereum-optimism/optimism/op-node/flags"
	p2pcli "github.com/ethereum-optimism/optimism/op-node/p2p/cli"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/driver"
	"github.com/ethereum-optimism/optimism/op-node/rollup/engine"
	"github.com/ethereum-optimism/optimism/op-node/rollup/interop"
	"github.com/ethereum-optimism/optimism/op-node/rollup/sync"
	opflags "github.com/ethereum-optimism/optimism/op-service/flags"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	"github.com/ethereum-optimism/optimism/op-service/rpc"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
)

// NewConfig creates a Config from the provided flags or environment variables.
func NewConfig(ctx *cli.Context, log log.Logger) (*config.Config, error) {
	if err := flags.CheckRequired(ctx); err != nil {
		return nil, err
	}

	rollupConfig, err := NewRollupConfigFromCLI(log, ctx)
	if err != nil {
		return nil, err
	}

	depSet, err := NewDependencySetFromCLI(ctx)
	if err != nil {
		return nil, err
	}

	if !ctx.Bool(flags.RollupLoadProtocolVersions.Name) {
		log.Info("Not opted in to ProtocolVersions signal loading, disabling ProtocolVersions contract now.")
		rollupConfig.ProtocolVersionsAddress = common.Address{}
	}

	configPersistence := NewConfigPersistence(ctx)

	driverConfig := NewDriverConfig(ctx)

	p2pSignerSetup, err := p2pcli.LoadSignerSetup(ctx, log)
	if err != nil {
		return nil, fmt.Errorf("failed to load p2p signer: %w", err)
	}

	p2pConfig, err := p2pcli.NewConfig(ctx, rollupConfig.BlockTime)
	if err != nil {
		return nil, fmt.Errorf("failed to load p2p config: %w", err)
	}

	l1Endpoint := NewL1EndpointConfig(ctx)

	l2Endpoint, err := NewL2EndpointConfig(ctx, log)
	if err != nil {
		return nil, fmt.Errorf("failed to load l2 endpoints info: %w", err)
	}

	syncConfig, err := NewSyncConfig(ctx, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create the sync config: %w", err)
	}

	haltOption := ctx.String(flags.RollupHalt.Name)
	if haltOption == "none" {
		haltOption = ""
	}

	if ctx.IsSet(flags.HeartbeatEnabledFlag.Name) ||
		ctx.IsSet(flags.HeartbeatMonikerFlag.Name) ||
		ctx.IsSet(flags.HeartbeatURLFlag.Name) {
		log.Warn("Heartbeat functionality is not supported anymore, CLI flags will be removed in following release.")
	}
	conductorRPCEndpoint := ctx.String(flags.ConductorRpcFlag.Name)
	cfg := &config.Config{
		L1:                          l1Endpoint,
		L2:                          l2Endpoint,
		Rollup:                      *rollupConfig,
		DependencySet:               depSet,
		Driver:                      *driverConfig,
		Beacon:                      NewBeaconEndpointConfig(ctx),
		InteropConfig:               NewSupervisorEndpointConfig(ctx),
		RPC:                         rpc.ReadCLIConfig(ctx),
		Metrics:                     opmetrics.ReadCLIConfig(ctx),
		Pprof:                       oppprof.ReadCLIConfig(ctx),
		P2P:                         p2pConfig,
		P2PSigner:                   p2pSignerSetup,
		L1EpochPollInterval:         ctx.Duration(flags.L1EpochPollIntervalFlag.Name),
		RuntimeConfigReloadInterval: ctx.Duration(flags.RuntimeConfigReloadIntervalFlag.Name),
		ConfigPersistence:           configPersistence,
		SafeDBPath:                  ctx.String(flags.SafeDBPath.Name),
		Sync:                        *syncConfig,
		RollupHalt:                  haltOption,

		ConductorEnabled: ctx.Bool(flags.ConductorEnabledFlag.Name),
		ConductorRpc: func(context.Context) (string, error) {
			return conductorRPCEndpoint, nil
		},
		ConductorRpcTimeout: ctx.Duration(flags.ConductorRpcTimeoutFlag.Name),

		AltDA: altda.ReadCLIConfig(ctx),

		IgnoreMissingPectraBlobSchedule: ctx.Bool(flags.IgnoreMissingPectraBlobSchedule.Name),
		FetchWithdrawalRootFromState:    ctx.Bool(flags.FetchWithdrawalRootFromState.Name),

		ExperimentalOPStackAPI: ctx.Bool(flags.ExperimentalOPStackAPI.Name),
	}

	if err := cfg.LoadPersisted(log); err != nil {
		return nil, fmt.Errorf("failed to load driver config: %w", err)
	}

	// conductor controls the sequencer state
	if cfg.ConductorEnabled {
		cfg.Driver.SequencerStopped = true
	}

	if err := cfg.Check(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func NewSupervisorEndpointConfig(ctx *cli.Context) *interop.Config {
	return &interop.Config{
		RPCAddr:          ctx.String(flags.InteropRPCAddr.Name),
		RPCPort:          ctx.Int(flags.InteropRPCPort.Name),
		RPCJwtSecretPath: ctx.String(flags.InteropJWTSecret.Name),
	}
}

func NewBeaconEndpointConfig(ctx *cli.Context) config.L1BeaconEndpointSetup {
	return &config.L1BeaconEndpointConfig{
		BeaconAddr:             ctx.String(flags.BeaconAddr.Name),
		BeaconHeader:           ctx.String(flags.BeaconHeader.Name),
		BeaconFallbackAddrs:    ctx.StringSlice(flags.BeaconFallbackAddrs.Name),
		BeaconCheckIgnore:      ctx.Bool(flags.BeaconCheckIgnore.Name),
		BeaconFetchAllSidecars: ctx.Bool(flags.BeaconFetchAllSidecars.Name),
	}
}

func NewL1EndpointConfig(ctx *cli.Context) *config.L1EndpointConfig {
	return &config.L1EndpointConfig{
		L1NodeAddr:       ctx.String(flags.L1NodeAddr.Name),
		L1TrustRPC:       ctx.Bool(flags.L1TrustRPC.Name),
		L1RPCKind:        sources.RPCProviderKind(strings.ToLower(ctx.String(flags.L1RPCProviderKind.Name))),
		RateLimit:        ctx.Float64(flags.L1RPCRateLimit.Name),
		BatchSize:        ctx.Int(flags.L1RPCMaxBatchSize.Name),
		HttpPollInterval: ctx.Duration(flags.L1HTTPPollInterval.Name),
		MaxConcurrency:   ctx.Int(flags.L1RPCMaxConcurrency.Name),
		CacheSize:        ctx.Uint(flags.L1CacheSize.Name),
	}
}

func NewL2EndpointConfig(ctx *cli.Context, logger log.Logger) (*config.L2EndpointConfig, error) {
	l2Addr := ctx.String(flags.L2EngineAddr.Name)
	fileName := ctx.String(flags.L2EngineJWTSecret.Name)
	secret, err := rpc.ObtainJWTSecret(logger, fileName, true)
	if err != nil {
		return nil, err
	}
	l2RpcTimeout := ctx.Duration(flags.L2EngineRpcTimeout.Name)
	return &config.L2EndpointConfig{
		L2EngineAddr:        l2Addr,
		L2EngineJWTSecret:   secret,
		L2EngineCallTimeout: l2RpcTimeout,
	}, nil
}

func NewConfigPersistence(ctx *cli.Context) config.ConfigPersistence {
	stateFile := ctx.String(flags.RPCAdminPersistence.Name)
	if stateFile == "" {
		return config.DisabledConfigPersistence{}
	}
	return config.NewConfigPersistence(stateFile)
}

func NewDriverConfig(ctx *cli.Context) *driver.Config {
	return &driver.Config{
		VerifierConfDepth:   ctx.Uint64(flags.VerifierL1Confs.Name),
		SequencerConfDepth:  ctx.Uint64(flags.SequencerL1Confs.Name),
		SequencerEnabled:    ctx.Bool(flags.SequencerEnabledFlag.Name),
		SequencerStopped:    ctx.Bool(flags.SequencerStoppedFlag.Name),
		SequencerMaxSafeLag: ctx.Uint64(flags.SequencerMaxSafeLagFlag.Name),
		RecoverMode:         ctx.Bool(flags.SequencerRecoverMode.Name),
	}
}

func NewRollupConfigFromCLI(log log.Logger, ctx *cli.Context) (*rollup.Config, error) {
	network := ctx.String(opflags.NetworkFlagName)
	rollupConfigPath := ctx.String(opflags.RollupConfigFlagName)
	if ctx.Bool(flags.BetaExtraNetworks.Name) {
		log.Warn("The beta.extra-networks flag is deprecated and can be omitted safely.")
	}
	rollupConfig, err := NewRollupConfig(log, network, rollupConfigPath)
	if err != nil {
		return nil, err
	}
	applyOverrides(ctx, rollupConfig)
	return rollupConfig, nil
}

func NewRollupConfig(log log.Logger, network string, rollupConfigPath string) (*rollup.Config, error) {
	if network != "" {
		if rollupConfigPath != "" {
			log.Error(`Cannot configure network and rollup-config at the same time.
Startup will proceed to use the network-parameter and ignore the rollup config.
Conflicting configuration is deprecated, and will stop the op-node from starting in the future.
`, "network", network, "rollup_config", rollupConfigPath)
		}
		rollupConfig, err := chaincfg.GetRollupConfig(network)
		if err != nil {
			return nil, err
		}
		return rollupConfig, nil
	}

	file, err := os.Open(rollupConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read rollup config: %w", err)
	}
	defer file.Close()

	var rollupConfig rollup.Config
	dec := json.NewDecoder(file)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&rollupConfig); err != nil {
		return nil, fmt.Errorf("failed to decode rollup config: %w", err)
	}
	return &rollupConfig, nil
}

func applyOverrides(ctx *cli.Context, rollupConfig *rollup.Config) {
	if ctx.IsSet(opflags.CanyonOverrideFlagName) {
		canyon := ctx.Uint64(opflags.CanyonOverrideFlagName)
		rollupConfig.CanyonTime = &canyon
	}
	if ctx.IsSet(opflags.DeltaOverrideFlagName) {
		delta := ctx.Uint64(opflags.DeltaOverrideFlagName)
		rollupConfig.DeltaTime = &delta
	}
	if ctx.IsSet(opflags.EcotoneOverrideFlagName) {
		ecotone := ctx.Uint64(opflags.EcotoneOverrideFlagName)
		rollupConfig.EcotoneTime = &ecotone
	}
	if ctx.IsSet(opflags.FjordOverrideFlagName) {
		fjord := ctx.Uint64(opflags.FjordOverrideFlagName)
		rollupConfig.FjordTime = &fjord
	}
	if ctx.IsSet(opflags.GraniteOverrideFlagName) {
		granite := ctx.Uint64(opflags.GraniteOverrideFlagName)
		rollupConfig.GraniteTime = &granite
	}
	if ctx.IsSet(opflags.HoloceneOverrideFlagName) {
		holocene := ctx.Uint64(opflags.HoloceneOverrideFlagName)
		rollupConfig.HoloceneTime = &holocene
	}
	if ctx.IsSet(opflags.PectraBlobScheduleOverrideFlagName) {
		pectrablobschedule := ctx.Uint64(opflags.PectraBlobScheduleOverrideFlagName)
		rollupConfig.PectraBlobScheduleTime = &pectrablobschedule
	}
	if ctx.IsSet(opflags.IsthmusOverrideFlagName) {
		isthmus := ctx.Uint64(opflags.IsthmusOverrideFlagName)
		rollupConfig.IsthmusTime = &isthmus
	}
	if ctx.IsSet(opflags.InteropOverrideFlagName) {
		interop := ctx.Uint64(opflags.InteropOverrideFlagName)
		rollupConfig.InteropTime = &interop
	}
}

func NewDependencySetFromCLI(ctx *cli.Context) (depset.DependencySet, error) {
	if !ctx.IsSet(flags.InteropDependencySet.Name) {
		return nil, nil
	}
	loader := &depset.JSONDependencySetLoader{Path: ctx.Path(flags.InteropDependencySet.Name)}
	return loader.LoadDependencySet(ctx.Context)
}

func NewSyncConfig(ctx *cli.Context, log log.Logger) (*sync.Config, error) {
	if ctx.IsSet(flags.L2EngineSyncEnabled.Name) && ctx.IsSet(flags.SyncModeFlag.Name) {
		return nil, errors.New("cannot set both --l2.engine-sync and --syncmode at the same time")
	} else if ctx.IsSet(flags.L2EngineSyncEnabled.Name) {
		log.Error("l2.engine-sync is deprecated and will be removed in a future release. Use --syncmode=execution-layer instead.")
	}
	mode, err := sync.StringToMode(ctx.String(flags.SyncModeFlag.Name))
	if err != nil {
		return nil, err
	}

	engineKind := engine.Kind(ctx.String(flags.L2EngineKind.Name))
	cfg := &sync.Config{
		SyncMode:                       mode,
		SkipSyncStartCheck:             ctx.Bool(flags.SkipSyncStartCheck.Name),
		SupportsPostFinalizationELSync: engineKind.SupportsPostFinalizationELSync(),
	}
	if ctx.Bool(flags.L2EngineSyncEnabled.Name) {
		cfg.SyncMode = sync.ELSync
	}

	return cfg, nil
}
