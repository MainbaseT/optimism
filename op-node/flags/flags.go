package flags

import (
	"fmt"
	"time"

	"github.com/urfave/cli/v2"

	altda "github.com/ethereum-optimism/optimism/op-alt-da"
	"github.com/ethereum-optimism/optimism/op-node/rollup/engine"
	"github.com/ethereum-optimism/optimism/op-node/rollup/sync"
	openum "github.com/ethereum-optimism/optimism/op-service/enum"
	opflags "github.com/ethereum-optimism/optimism/op-service/flags"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
	"github.com/ethereum-optimism/optimism/op-service/sources"
)

// Flags

const EnvVarPrefix = "OP_NODE"

const (
	RollupCategory     = "1. ROLLUP"
	L1RPCCategory      = "2. L1 RPC"
	SequencerCategory  = "3. SEQUENCER"
	OperationsCategory = "4. LOGGING, METRICS, DEBUGGING, AND API"
	P2PCategory        = "5. PEER-TO-PEER"
	AltDACategory      = "6. ALT-DA (EXPERIMENTAL)"
	MiscCategory       = "7. MISC"
	InteropCategory    = "8. INTEROP (SUPER EXPERIMENTAL)"
)

func init() {
	cli.HelpFlag.(*cli.BoolFlag).Category = MiscCategory
	cli.VersionFlag.(*cli.BoolFlag).Category = MiscCategory
}

func prefixEnvVars(names ...string) []string {
	envs := make([]string, 0, len(names))
	for _, name := range names {
		envs = append(envs, EnvVarPrefix+"_"+name)
	}
	return envs
}

var (
	/* Required Flags */
	L1NodeAddr = &cli.StringFlag{
		Name:     "l1",
		Usage:    "Address of L1 User JSON-RPC endpoint to use (eth namespace required)",
		Value:    "http://127.0.0.1:8545",
		EnvVars:  prefixEnvVars("L1_ETH_RPC"),
		Category: RollupCategory,
	}
	L2EngineAddr = &cli.StringFlag{
		Name:     "l2",
		Usage:    "Address of L2 Engine JSON-RPC endpoints to use (engine and eth namespace required)",
		EnvVars:  prefixEnvVars("L2_ENGINE_RPC"),
		Category: RollupCategory,
	}
	L2EngineJWTSecret = &cli.StringFlag{
		Name:        "l2.jwt-secret",
		Usage:       "Path to JWT secret key. Keys are 32 bytes, hex encoded in a file. A new key will be generated if the file is empty.",
		EnvVars:     prefixEnvVars("L2_ENGINE_AUTH"),
		Value:       "",
		Destination: new(string),
		Category:    RollupCategory,
	}
	BeaconAddr = &cli.StringFlag{
		Name:     "l1.beacon",
		Usage:    "Address of L1 Beacon-node HTTP endpoint to use.",
		Required: false,
		EnvVars:  prefixEnvVars("L1_BEACON"),
		Category: RollupCategory,
	}
	/* Optional Flags */
	BeaconHeader = &cli.StringFlag{
		Name:     "l1.beacon-header",
		Usage:    "Optional HTTP header to add to all requests to the L1 Beacon endpoint. Format: 'X-Key: Value'",
		Required: false,
		EnvVars:  prefixEnvVars("L1_BEACON_HEADER"),
		Category: L1RPCCategory,
	}
	BeaconFallbackAddrs = &cli.StringSliceFlag{
		Name:     "l1.beacon-fallbacks",
		Aliases:  []string{"l1.beacon-archiver"},
		Usage:    "Addresses of L1 Beacon-API compatible HTTP fallback endpoints. Used to fetch blob sidecars not available at the l1.beacon (e.g. expired blobs).",
		EnvVars:  prefixEnvVars("L1_BEACON_FALLBACKS", "L1_BEACON_ARCHIVER"),
		Category: L1RPCCategory,
	}
	BeaconCheckIgnore = &cli.BoolFlag{
		Name:     "l1.beacon.ignore",
		Usage:    "When false, halts op-node startup if the healthcheck to the Beacon-node endpoint fails.",
		Required: false,
		Value:    false,
		EnvVars:  prefixEnvVars("L1_BEACON_IGNORE"),
		Category: L1RPCCategory,
	}
	BeaconFetchAllSidecars = &cli.BoolFlag{
		Name:     "l1.beacon.fetch-all-sidecars",
		Usage:    "If true, all sidecars are fetched and filtered locally. Workaround for buggy Beacon nodes.",
		Required: false,
		Value:    false,
		EnvVars:  prefixEnvVars("L1_BEACON_FETCH_ALL_SIDECARS"),
		Category: L1RPCCategory,
	}
	SyncModeFlag = &cli.GenericFlag{
		Name:    "syncmode",
		Usage:   fmt.Sprintf("Blockchain sync mode (options: %s)", openum.EnumString(sync.ModeStrings)),
		EnvVars: prefixEnvVars("SYNCMODE"),
		Value: func() *sync.Mode {
			out := sync.CLSync
			return &out
		}(),
		Category: RollupCategory,
	}
	RPCAdminPersistence = &cli.StringFlag{
		Name:     "rpc.admin-state",
		Usage:    "File path used to persist state changes made via the admin API so they persist across restarts. Disabled if not set.",
		EnvVars:  prefixEnvVars("RPC_ADMIN_STATE"),
		Category: OperationsCategory,
	}
	FetchWithdrawalRootFromState = &cli.BoolFlag{
		Name:     "fetch-withdrawal-root-from-state",
		Usage:    "Read withdrawal_storage_root (aka message passer storage root) from state trie (via execution layer) instead of the block header. Restores pre-Isthmus behavior, requires an archive EL client.",
		Required: false,
		EnvVars:  prefixEnvVars("FETCH_WITHDRAWAL_ROOT_FROM_STATE"),
		Category: OperationsCategory,
	}
	L1TrustRPC = &cli.BoolFlag{
		Name:     "l1.trustrpc",
		Usage:    "Trust the L1 RPC, sync faster at risk of malicious/buggy RPC providing bad or inconsistent L1 data",
		EnvVars:  prefixEnvVars("L1_TRUST_RPC"),
		Category: L1RPCCategory,
	}
	L1RPCProviderKind = &cli.GenericFlag{
		Name: "l1.rpckind",
		Usage: "The kind of RPC provider, used to inform optimal transactions receipts fetching, and thus reduce costs. Valid options: " +
			openum.EnumString(sources.RPCProviderKinds),
		EnvVars: prefixEnvVars("L1_RPC_KIND"),
		Value: func() *sources.RPCProviderKind {
			out := sources.RPCKindStandard
			return &out
		}(),
		Category: L1RPCCategory,
	}
	L1RPCMaxConcurrency = &cli.IntFlag{
		Name:     "l1.max-concurrency",
		Usage:    "Maximum number of concurrent RPC requests to make to the L1 RPC provider.",
		EnvVars:  prefixEnvVars("L1_MAX_CONCURRENCY"),
		Value:    10,
		Category: L1RPCCategory,
	}
	L1RPCRateLimit = &cli.Float64Flag{
		Name:     "l1.rpc-rate-limit",
		Usage:    "Optional self-imposed global rate-limit on L1 RPC requests, specified in requests / second. Disabled if set to 0.",
		EnvVars:  prefixEnvVars("L1_RPC_RATE_LIMIT"),
		Value:    0,
		Category: L1RPCCategory,
	}
	L1RPCMaxBatchSize = &cli.IntFlag{
		Name:     "l1.rpc-max-batch-size",
		Usage:    "Maximum number of RPC requests to bundle, e.g. during L1 blocks receipt fetching. The L1 RPC rate limit counts this as N items, but allows it to burst at once.",
		EnvVars:  prefixEnvVars("L1_RPC_MAX_BATCH_SIZE"),
		Value:    20,
		Category: L1RPCCategory,
	}
	L1CacheSize = &cli.UintFlag{
		Name: "l1.cache-size",
		Usage: "Cache size for blocks, receipts and transactions. " +
			"If this flag is set to 0, 2/3 of the sequencing window size is used (usually 2400). " +
			"The default value of 900 (~3h of L1 blocks) is good for (high-throughput) networks that see frequent safe head increments. " +
			"On (low-throughput) networks with infrequent safe head increments, it is recommended to set this value to 0, " +
			"or a value that well covers the typical span between safe head increments. " +
			"Note that higher values will cause significantly increased memory usage.",
		EnvVars:  prefixEnvVars("L1_CACHE_SIZE"),
		Value:    900, // ~3h of L1 blocks
		Category: L1RPCCategory,
	}
	L1HTTPPollInterval = &cli.DurationFlag{
		Name:     "l1.http-poll-interval",
		Usage:    "Polling interval for latest-block subscription when using an HTTP RPC provider. Ignored for other types of RPC endpoints.",
		EnvVars:  prefixEnvVars("L1_HTTP_POLL_INTERVAL"),
		Value:    time.Second * 12,
		Category: L1RPCCategory,
	}
	L2EngineKind = &cli.GenericFlag{
		Name: "l2.enginekind",
		Usage: "The kind of engine client, used to control the behavior of optimism in respect to different types of engine clients. Valid options: " +
			openum.EnumString(engine.Kinds),
		EnvVars: prefixEnvVars("L2_ENGINE_KIND"),
		Value: func() *engine.Kind {
			out := engine.Geth
			return &out
		}(),
		Category: RollupCategory,
	}
	L2EngineRpcTimeout = &cli.DurationFlag{
		Name:     "l2.engine-rpc-timeout",
		Usage:    "L2 engine client rpc timeout",
		EnvVars:  prefixEnvVars("L2_ENGINE_RPC_TIMEOUT"),
		Value:    time.Second * 10,
		Category: RollupCategory,
	}
	VerifierL1Confs = &cli.Uint64Flag{
		Name:     "verifier.l1-confs",
		Usage:    "Number of L1 blocks to keep distance from the L1 head before deriving L2 data from. Reorgs are supported, but may be slow to perform.",
		EnvVars:  prefixEnvVars("VERIFIER_L1_CONFS"),
		Value:    0,
		Category: L1RPCCategory,
	}
	SequencerEnabledFlag = &cli.BoolFlag{
		Name:     "sequencer.enabled",
		Usage:    "Enable sequencing of new L2 blocks. A separate batch submitter has to be deployed to publish the data for verifiers.",
		EnvVars:  prefixEnvVars("SEQUENCER_ENABLED"),
		Category: SequencerCategory,
	}
	SequencerStoppedFlag = &cli.BoolFlag{
		Name:     "sequencer.stopped",
		Usage:    "Initialize the sequencer in a stopped state. The sequencer can be started using the admin_startSequencer RPC",
		EnvVars:  prefixEnvVars("SEQUENCER_STOPPED"),
		Category: SequencerCategory,
	}
	SequencerMaxSafeLagFlag = &cli.Uint64Flag{
		Name:     "sequencer.max-safe-lag",
		Usage:    "Maximum number of L2 blocks for restricting the distance between L2 safe and unsafe. Disabled if 0.",
		EnvVars:  prefixEnvVars("SEQUENCER_MAX_SAFE_LAG"),
		Value:    0,
		Category: SequencerCategory,
	}
	SequencerL1Confs = &cli.Uint64Flag{
		Name:     "sequencer.l1-confs",
		Usage:    "Number of L1 blocks to keep distance from the L1 head as a sequencer for picking an L1 origin.",
		EnvVars:  prefixEnvVars("SEQUENCER_L1_CONFS"),
		Value:    4,
		Category: SequencerCategory,
	}
	SequencerRecoverMode = &cli.BoolFlag{
		Name:     "sequencer.recover",
		Usage:    "Forces the sequencer to strictly prepare the next L1 origin and create empty L2 blocks",
		EnvVars:  prefixEnvVars("SEQUENCER_RECOVER"),
		Value:    false,
		Category: SequencerCategory,
	}
	L1EpochPollIntervalFlag = &cli.DurationFlag{
		Name:     "l1.epoch-poll-interval",
		Usage:    "Poll interval for retrieving new L1 epoch updates such as safe and finalized block changes. Disabled if 0 or negative.",
		EnvVars:  prefixEnvVars("L1_EPOCH_POLL_INTERVAL"),
		Value:    time.Second * 12 * 32,
		Category: L1RPCCategory,
	}
	RuntimeConfigReloadIntervalFlag = &cli.DurationFlag{
		Name:     "l1.runtime-config-reload-interval",
		Usage:    "Poll interval for reloading the runtime config, useful when config events are not being picked up. Disabled if 0 or negative.",
		EnvVars:  prefixEnvVars("L1_RUNTIME_CONFIG_RELOAD_INTERVAL"),
		Value:    time.Minute * 10,
		Category: L1RPCCategory,
	}
	SnapshotLog = &cli.StringFlag{
		Name:     "snapshotlog.file",
		Usage:    "Deprecated. This flag is ignored, but here for compatibility.",
		EnvVars:  prefixEnvVars("SNAPSHOT_LOG"),
		Category: OperationsCategory,
		Hidden:   true, // non-critical function, removed, flag is no-op to avoid breaking setups.
	}
	HeartbeatEnabledFlag = &cli.BoolFlag{
		Name:     "heartbeat.enabled",
		Usage:    "Deprecated, no-op flag.",
		EnvVars:  prefixEnvVars("HEARTBEAT_ENABLED"),
		Category: OperationsCategory,
		Hidden:   true,
	}
	HeartbeatMonikerFlag = &cli.StringFlag{
		Name:     "heartbeat.moniker",
		Usage:    "Deprecated, no-op flag.",
		EnvVars:  prefixEnvVars("HEARTBEAT_MONIKER"),
		Category: OperationsCategory,
		Hidden:   true,
	}
	HeartbeatURLFlag = &cli.StringFlag{
		Name:     "heartbeat.url",
		Usage:    "Deprecated, no-op flag.",
		EnvVars:  prefixEnvVars("HEARTBEAT_URL"),
		Category: OperationsCategory,
		Hidden:   true,
	}
	RollupHalt = &cli.StringFlag{
		Name:     "rollup.halt",
		Usage:    "Opt-in option to halt on incompatible protocol version requirements of the given level (major/minor/patch/none), as signaled onchain in L1",
		EnvVars:  prefixEnvVars("ROLLUP_HALT"),
		Category: RollupCategory,
	}
	RollupLoadProtocolVersions = &cli.BoolFlag{
		Name:     "rollup.load-protocol-versions",
		Usage:    "Load protocol versions from the superchain L1 ProtocolVersions contract (if available), and report in logs and metrics",
		EnvVars:  prefixEnvVars("ROLLUP_LOAD_PROTOCOL_VERSIONS"),
		Category: RollupCategory,
	}
	SafeDBPath = &cli.StringFlag{
		Name:     "safedb.path",
		Usage:    "File path used to persist safe head update data. Disabled if not set.",
		EnvVars:  prefixEnvVars("SAFEDB_PATH"),
		Category: OperationsCategory,
	}
	/* Deprecated Flags */
	L2EngineSyncEnabled = &cli.BoolFlag{
		Name:    "l2.engine-sync",
		Usage:   "WARNING: Deprecated. Use --syncmode=execution-layer instead",
		EnvVars: prefixEnvVars("L2_ENGINE_SYNC_ENABLED"),
		Value:   false,
		Hidden:  true,
	}
	SkipSyncStartCheck = &cli.BoolFlag{
		Name: "l2.skip-sync-start-check",
		Usage: "Skip sanity check of consistency of L1 origins of the unsafe L2 blocks when determining the sync-starting point. " +
			"This defers the L1-origin verification, and is recommended to use in when utilizing l2.engine-sync",
		EnvVars: prefixEnvVars("L2_SKIP_SYNC_START_CHECK"),
		Value:   false,
		Hidden:  true,
	}
	BetaExtraNetworks = &cli.BoolFlag{
		Name:    "beta.extra-networks",
		Usage:   "Legacy flag, ignored, all superchain-registry networks are enabled by default.",
		EnvVars: prefixEnvVars("BETA_EXTRA_NETWORKS"),
		Hidden:  true, // hidden, this is deprecated, the flag is not used anymore.
	}
	BackupL2UnsafeSyncRPC = &cli.StringFlag{
		Name:    "l2.backup-unsafe-sync-rpc",
		Usage:   "Set the backup L2 unsafe sync RPC endpoint.",
		EnvVars: prefixEnvVars("L2_BACKUP_UNSAFE_SYNC_RPC"),
		Hidden:  true,
	}
	BackupL2UnsafeSyncRPCTrustRPC = &cli.StringFlag{
		Name: "l2.backup-unsafe-sync-rpc.trustrpc",
		Usage: "Like l1.trustrpc, configure if response data from the RPC needs to be verified, e.g. blockhash computation." +
			"This does not include checks if the blockhash is part of the canonical chain.",
		EnvVars: prefixEnvVars("L2_BACKUP_UNSAFE_SYNC_RPC_TRUST_RPC"),
		Hidden:  true,
	}
	ConductorEnabledFlag = &cli.BoolFlag{
		Name:     "conductor.enabled",
		Usage:    "Enable the conductor service",
		EnvVars:  prefixEnvVars("CONDUCTOR_ENABLED"),
		Value:    false,
		Category: SequencerCategory,
	}
	ConductorRpcFlag = &cli.StringFlag{
		Name:     "conductor.rpc",
		Usage:    "Conductor service rpc endpoint",
		EnvVars:  prefixEnvVars("CONDUCTOR_RPC"),
		Value:    "http://127.0.0.1:8547",
		Category: SequencerCategory,
	}
	ConductorRpcTimeoutFlag = &cli.DurationFlag{
		Name:     "conductor.rpc-timeout",
		Usage:    "Conductor service rpc timeout",
		EnvVars:  prefixEnvVars("CONDUCTOR_RPC_TIMEOUT"),
		Value:    time.Second * 1,
		Category: SequencerCategory,
	}
	/* Interop flags, experimental. */
	InteropRPCAddr = &cli.StringFlag{
		Name: "interop.rpc.addr",
		Usage: "Interop Websocket-only RPC listening address, for supervisor service to manage syncing of the op-node." +
			"Applies only to Interop-enabled networks. Optional, disabled if left empty. " +
			"Do not enable if you do not run a supervisor service.",
		EnvVars:  prefixEnvVars("INTEROP_RPC_ADDR"),
		Value:    "",
		Category: InteropCategory,
	}
	InteropRPCPort = &cli.IntFlag{
		Name: "interop.rpc.port",
		Usage: "Interop RPC listening port, to serve supervisor syncing." +
			"Applies only to Interop-enabled networks.",
		EnvVars:  prefixEnvVars("INTEROP_RPC_PORT"),
		Value:    9645, // Note: op-service/rpc/cli.go uses 8545 as the default.
		Category: InteropCategory,
	}
	InteropJWTSecret = &cli.StringFlag{
		Name: "interop.jwt-secret",
		Usage: "Interop RPC server authentication. Path to JWT secret key. Keys are 32 bytes, hex encoded in a file. " +
			"A new key will be generated if the file is empty. " +
			"Applies only to Interop-enabled networks.",
		EnvVars:     prefixEnvVars("INTEROP_JWT_SECRET"),
		Value:       "",
		Destination: new(string),
		Category:    InteropCategory,
	}
	InteropDependencySet = &cli.PathFlag{
		Name:      "interop.dependency-set",
		Usage:     "Dependency-set configuration, point at JSON file.",
		EnvVars:   prefixEnvVars("INTEROP_DEPENDENCY_SET"),
		TakesFile: true,
		Category:  InteropCategory,
	}

	IgnoreMissingPectraBlobSchedule = &cli.BoolFlag{
		Name: "ignore-missing-pectra-blob-schedule",
		Usage: "Ignore missing pectra blob schedule fix for Sepolia and Holesky chains. Only set if you know what you are doing!" +
			"Ask your chain's operator for the correct Pectra blob schedule activation time and set it via the rollup.json config" +
			"or use the --override.pectrablobschedule flag.",
		EnvVars:  prefixEnvVars("IGNORE_MISSING_PECTRA_BLOB_SCHEDULE"),
		Category: RollupCategory,
		Hidden:   true,
	}

	ExperimentalOPStackAPI = &cli.BoolFlag{
		Name:     "experimental.sequencer-api",
		Usage:    "Enables experimental test sequencer RPC functionality",
		Required: false,
		EnvVars:  prefixEnvVars("EXPERIMENTAL_SEQUENCER_API"),
		Category: MiscCategory,
	}
)

var requiredFlags = []cli.Flag{
	L1NodeAddr,
	L2EngineAddr,
	L2EngineJWTSecret,
}

var optionalFlags = []cli.Flag{
	BeaconAddr,
	BeaconHeader,
	BeaconFallbackAddrs,
	BeaconCheckIgnore,
	BeaconFetchAllSidecars,
	SyncModeFlag,
	FetchWithdrawalRootFromState,
	L1TrustRPC,
	L1RPCProviderKind,
	L1RPCRateLimit,
	L1RPCMaxBatchSize,
	L1RPCMaxConcurrency,
	L1HTTPPollInterval,
	L1CacheSize,
	VerifierL1Confs,
	SequencerEnabledFlag,
	SequencerStoppedFlag,
	SequencerMaxSafeLagFlag,
	SequencerL1Confs,
	SequencerRecoverMode,
	L1EpochPollIntervalFlag,
	RuntimeConfigReloadIntervalFlag,
	RPCAdminPersistence,
	SnapshotLog,
	HeartbeatEnabledFlag,
	HeartbeatMonikerFlag,
	HeartbeatURLFlag,
	RollupHalt,
	RollupLoadProtocolVersions,
	ConductorEnabledFlag,
	ConductorRpcFlag,
	ConductorRpcTimeoutFlag,
	SafeDBPath,
	L2EngineKind,
	L2EngineRpcTimeout,
	InteropRPCAddr,
	InteropRPCPort,
	InteropJWTSecret,
	InteropDependencySet,
	IgnoreMissingPectraBlobSchedule,
	ExperimentalOPStackAPI,
}

var DeprecatedFlags = []cli.Flag{
	L2EngineSyncEnabled,
	SkipSyncStartCheck,
	BetaExtraNetworks,
	BackupL2UnsafeSyncRPC,
	BackupL2UnsafeSyncRPCTrustRPC,
	// Deprecated P2P Flags are added at the init step
}

// Flags contains the list of configuration options available to the binary.
var Flags []cli.Flag

var rpcDefaults = oprpc.CLIConfig{
	ListenAddr:  "0.0.0.0", // TODO(#16487): Switch to 127.0.0.1
	ListenPort:  9545,      // op-node defaults to a different port than ethereum EL (8545)
	EnableAdmin: false,
}

func init() {
	DeprecatedFlags = append(DeprecatedFlags, deprecatedP2PFlags(EnvVarPrefix)...)
	optionalFlags = append(optionalFlags, P2PFlags(EnvVarPrefix)...)
	optionalFlags = append(optionalFlags, oplog.CLIFlagsWithCategory(EnvVarPrefix, OperationsCategory)...)
	optionalFlags = append(optionalFlags, oppprof.CLIFlagsWithCategory(EnvVarPrefix, OperationsCategory)...)
	optionalFlags = append(optionalFlags, opmetrics.CLIFlagsWithCategory(EnvVarPrefix, OperationsCategory)...)
	optionalFlags = append(optionalFlags, oprpc.CLIFlagsWithCategory(EnvVarPrefix, OperationsCategory, rpcDefaults)...)
	optionalFlags = append(optionalFlags, DeprecatedFlags...)
	optionalFlags = append(optionalFlags, opflags.CLIFlags(EnvVarPrefix, RollupCategory)...)
	optionalFlags = append(optionalFlags, altda.CLIFlags(EnvVarPrefix, AltDACategory)...)
	Flags = append(requiredFlags, optionalFlags...)
}

func CheckRequired(ctx *cli.Context) error {
	for _, f := range requiredFlags {
		if !ctx.IsSet(f.Names()[0]) {
			return fmt.Errorf("flag %s is required", f.Names()[0])
		}
	}
	return opflags.CheckRequiredXor(ctx)
}
