package config

import (
	"errors"

	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/syncnode"
)

var (
	ErrMissingSyncSources   = errors.New("must specify sync source collection")
	ErrMissingFullConfigSet = errors.New("must specify a full config set source")
	ErrMissingDatadir       = errors.New("must specify datadir")
)

type Config struct {
	Version string

	LogConfig     oplog.CLIConfig
	MetricsConfig opmetrics.CLIConfig
	PprofConfig   oppprof.CLIConfig
	RPC           oprpc.CLIConfig

	FullConfigSetSource depset.FullConfigSetSource

	// MockRun runs the service with a mock backend
	MockRun bool

	// SynchronousProcessors disables background-workers,
	// requiring manual triggers for the backend to process anything.
	SynchronousProcessors bool

	L1RPC string

	// SyncSources lists the consensus nodes that help sync the supervisor
	SyncSources syncnode.SyncNodeCollection

	Datadir             string
	DatadirSyncEndpoint string

	// RPCVerificationWarnings enables asynchronous RPC verification of DB checkAccess call in the CheckAccessList endpoint, indicating warnings as a metric
	RPCVerificationWarnings bool

	// FailsafeEnabled enables failsafe mode for the supervisor
	FailsafeEnabled bool

	// FailsafeOnInvalidation controls whether failsafe should activate when a block is invalidated
	FailsafeOnInvalidation bool
}

func (c *Config) Check() error {
	var result error
	result = errors.Join(result, c.MetricsConfig.Check())
	result = errors.Join(result, c.PprofConfig.Check())
	result = errors.Join(result, c.RPC.Check())
	if c.FullConfigSetSource == nil {
		result = errors.Join(result, ErrMissingFullConfigSet)
	}
	if c.Datadir == "" {
		result = errors.Join(result, ErrMissingDatadir)
	}
	if c.SyncSources == nil {
		result = errors.Join(result, ErrMissingSyncSources)
	} else {
		result = errors.Join(result, c.SyncSources.Check())
	}
	return result
}

// NewConfig creates a new config using default values whenever possible.
// Required options with no suitable default are passed as parameters.
func NewConfig(l1RPC string, syncSrcs syncnode.SyncNodeCollection, fullCfgSet depset.FullConfigSetSource, datadir string) *Config {
	return &Config{
		LogConfig:              oplog.DefaultCLIConfig(),
		MetricsConfig:          opmetrics.DefaultCLIConfig(),
		PprofConfig:            oppprof.DefaultCLIConfig(),
		RPC:                    oprpc.DefaultCLIConfig(),
		FullConfigSetSource:    fullCfgSet,
		MockRun:                false,
		L1RPC:                  l1RPC,
		SyncSources:            syncSrcs,
		Datadir:                datadir,
		FailsafeEnabled:        false,
		FailsafeOnInvalidation: true,
	}
}
