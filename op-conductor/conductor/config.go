package conductor

import (
	"fmt"
	"math"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"

	"github.com/ethereum-optimism/optimism/op-conductor/flags"
	opnode "github.com/ethereum-optimism/optimism/op-node"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
)

type Config struct {
	// ConsensusAddr is the address, excluding port, to listen on for consensus connections.
	// E.g. 0.0.0.0 to bind to the external-facing network interface.
	ConsensusAddr string

	// ConsensusPort is the port to listen on for consensus connections.
	// If 0, the server binds to a port selected by the system.
	ConsensusPort int

	// ConsensusAdvertisedAddr is the network address, including port, to advertise to other peers.
	// This is optional: if empty, the address that the server network transport binds to is used instead.
	// E.g. local tests may use temporary addresses, rather than preset known addresses.
	ConsensusAdvertisedAddr string

	// RaftServerID is the unique ID for this server used by raft consensus.
	RaftServerID string

	// RaftStorageDir is the directory to store raft data.
	RaftStorageDir string

	// RaftBootstrap is true if this node should bootstrap a new raft cluster.
	RaftBootstrap bool

	// RaftSnapshotInterval is the interval to check if a snapshot should be taken.
	RaftSnapshotInterval time.Duration

	// RaftSnapshotThreshold is the number of logs to trigger a snapshot.
	RaftSnapshotThreshold uint64

	// RaftTrailingLogs is the number of logs to keep after a snapshot.
	RaftTrailingLogs uint64

	// RaftHeartbeatTimeout is the interval timeout between leader and followers.
	RaftHeartbeatTimeout time.Duration

	// RaftLeaderLeaseTimeout is the timeout for leader lease.
	// If the leader reaches this timeout without contacts to followers, it resigns.
	RaftLeaderLeaseTimeout time.Duration

	// NodeRPC is the HTTP provider URL for op-node.
	NodeRPC string

	// ExecutionRPC is the HTTP provider URL for execution layer.
	ExecutionRPC string

	// SupervisorRPC is the HTTP provider URL for supervisor.
	SupervisorRPC string

	// RollupBoostEnabled is true if the rollup boost is enabled.
	RollupBoostEnabled bool

	// RollupBoostHealthcheckTimeout is the timeout for rollup boost healthcheck.
	RollupBoostHealthcheckTimeout time.Duration

	// Paused is true if the conductor should start in a paused state.
	Paused bool

	// HealthCheck is the health check configuration.
	HealthCheck HealthCheckConfig

	// RollupCfg is the rollup config.
	RollupCfg rollup.Config

	// RPCEnableProxy is true if the sequencer RPC proxy should be enabled.
	RPCEnableProxy bool

	// The following fields are used to configure the websocket server that op-conductor exposes to get flashblocks from rollup boost and send them to clients.
	// RollupBoostWsURL is the URL of the rollup boost websocket proxy.
	RollupBoostWsURL string

	// WebsocketServerPort is the port at which op-conductor exposes its websocket server from which clients can read streams sourced from rollupBoostWsUrl.
	WebsocketServerPort int

	LogConfig     oplog.CLIConfig
	MetricsConfig opmetrics.CLIConfig
	PprofConfig   oppprof.CLIConfig
	RPC           oprpc.CLIConfig
}

// Check validates the CLIConfig.
func (c *Config) Check() error {
	if c.ConsensusAddr == "" {
		return fmt.Errorf("missing consensus address")
	}
	if c.ConsensusPort < 0 || c.ConsensusPort > math.MaxUint16 {
		return fmt.Errorf("invalid RPC port")
	}
	if c.RaftServerID == "" {
		return fmt.Errorf("missing raft server ID")
	}
	if c.RaftStorageDir == "" {
		return fmt.Errorf("missing raft storage directory")
	}
	if c.NodeRPC == "" {
		return fmt.Errorf("missing node RPC")
	}
	if c.ExecutionRPC == "" {
		return fmt.Errorf("missing geth RPC")
	}
	if err := c.HealthCheck.Check(); err != nil {
		return errors.Wrap(err, "invalid health check config")
	}
	if err := c.RollupCfg.Check(); err != nil {
		return errors.Wrap(err, "invalid rollup config")
	}
	if err := c.MetricsConfig.Check(); err != nil {
		return errors.Wrap(err, "invalid metrics config")
	}
	if err := c.PprofConfig.Check(); err != nil {
		return errors.Wrap(err, "invalid pprof config")
	}
	if err := c.RPC.Check(); err != nil {
		return errors.Wrap(err, "invalid rpc config")
	}
	return nil
}

// NewConfig parses the Config from the provided flags or environment variables.
func NewConfig(ctx *cli.Context, log log.Logger) (*Config, error) {
	if err := flags.CheckRequired(ctx); err != nil {
		return nil, errors.Wrap(err, "missing required flags")
	}

	rollupCfg, err := opnode.NewRollupConfigFromCLI(log, ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load rollup config")
	}

	executionP2pRpcUrl := ctx.String(flags.HealthcheckExecutionP2pRPCUrl.Name)
	if executionP2pRpcUrl == "" {
		executionP2pRpcUrl = ctx.String(flags.ExecutionRPC.Name)
	}

	return &Config{
		ConsensusAddr: ctx.String(flags.ConsensusAddr.Name),
		ConsensusPort: ctx.Int(flags.ConsensusPort.Name),
		// The consensus server will advertise the address it binds to if this is empty/unspecified.
		ConsensusAdvertisedAddr: ctx.String(flags.AdvertisedFullAddr.Name),

		RaftBootstrap:                 ctx.Bool(flags.RaftBootstrap.Name),
		RaftServerID:                  ctx.String(flags.RaftServerID.Name),
		RaftStorageDir:                ctx.String(flags.RaftStorageDir.Name),
		RaftSnapshotInterval:          ctx.Duration(flags.RaftSnapshotInterval.Name),
		RaftSnapshotThreshold:         ctx.Uint64(flags.RaftSnapshotThreshold.Name),
		RaftTrailingLogs:              ctx.Uint64(flags.RaftTrailingLogs.Name),
		RaftHeartbeatTimeout:          ctx.Duration(flags.RaftHeartbeatTimeout.Name),
		RaftLeaderLeaseTimeout:        ctx.Duration(flags.RaftLeaderLeaseTimeout.Name),
		NodeRPC:                       ctx.String(flags.NodeRPC.Name),
		ExecutionRPC:                  ctx.String(flags.ExecutionRPC.Name),
		SupervisorRPC:                 ctx.String(flags.SupervisorRPC.Name),
		RollupBoostEnabled:            ctx.Bool(flags.RollupBoostEnabled.Name),
		RollupBoostHealthcheckTimeout: ctx.Duration(flags.RollupBoostHealthcheckTimeout.Name),
		Paused:                        ctx.Bool(flags.Paused.Name),
		HealthCheck: HealthCheckConfig{
			Interval:                 ctx.Uint64(flags.HealthCheckInterval.Name),
			UnsafeInterval:           ctx.Uint64(flags.HealthCheckUnsafeInterval.Name),
			SafeEnabled:              ctx.Bool(flags.HealthCheckSafeEnabled.Name),
			SafeInterval:             ctx.Uint64(flags.HealthCheckSafeInterval.Name),
			MinPeerCount:             ctx.Uint64(flags.HealthCheckMinPeerCount.Name),
			ExecutionP2pEnabled:      ctx.Bool(flags.HealthcheckExecutionP2pEnabled.Name),
			ExecutionP2pMinPeerCount: ctx.Uint64(flags.HealthcheckExecutionP2pMinPeerCount.Name),
			ExecutionP2pRPCUrl:       executionP2pRpcUrl,
		},
		RollupCfg:           *rollupCfg,
		RPCEnableProxy:      ctx.Bool(flags.RPCEnableProxy.Name),
		RollupBoostWsURL:    ctx.String(flags.RollupBoostWsURL.Name),
		WebsocketServerPort: ctx.Int(flags.WebsocketServerPort.Name),
		LogConfig:           oplog.ReadCLIConfig(ctx),
		MetricsConfig:       opmetrics.ReadCLIConfig(ctx),
		PprofConfig:         oppprof.ReadCLIConfig(ctx),
		RPC:                 oprpc.ReadCLIConfig(ctx),
	}, nil
}

// HealthCheckConfig defines health check configuration.
type HealthCheckConfig struct {
	// Interval is the interval (in seconds) to check the health of the sequencer.
	Interval uint64

	// UnsafeInterval is the interval allowed between unsafe head and now in seconds.
	UnsafeInterval uint64

	// SafeEnabled is whether to enable safe head progression checks.
	SafeEnabled bool

	// SafeInterval is the interval between safe head progression measured in seconds.
	SafeInterval uint64

	// MinPeerCount is the minimum number of peers required for the sequencer to be healthy.
	MinPeerCount uint64

	// ExecutionP2pEnabled is whether to enable EL P2P checks.
	ExecutionP2pEnabled bool

	// ExecutionP2pRPC is the HTTP provider URL for EL P2P.
	ExecutionP2pRPCUrl string

	// ExecutionP2pMinPeerCount is the minimum number of EL P2P peers required for the sequencer to be healthy.
	ExecutionP2pMinPeerCount uint64
}

func (c *HealthCheckConfig) Check() error {
	if c.Interval == 0 {
		return fmt.Errorf("missing health check interval")
	}
	if c.SafeInterval == 0 {
		return fmt.Errorf("missing safe interval")
	}
	if c.MinPeerCount == 0 {
		return fmt.Errorf("missing minimum peer count")
	}
	if c.ExecutionP2pEnabled {
		if c.ExecutionP2pMinPeerCount == 0 {
			return fmt.Errorf("missing minimum el p2p peers")
		}
		if c.ExecutionP2pRPCUrl == "" {
			return fmt.Errorf("missing el p2p rpc")
		}
	}
	return nil
}
