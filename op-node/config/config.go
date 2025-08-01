package config

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/log"

	altda "github.com/ethereum-optimism/optimism/op-alt-da"
	"github.com/ethereum-optimism/optimism/op-node/flags"
	"github.com/ethereum-optimism/optimism/op-node/node/tracer"
	"github.com/ethereum-optimism/optimism/op-node/p2p"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/driver"
	"github.com/ethereum-optimism/optimism/op-node/rollup/interop"
	"github.com/ethereum-optimism/optimism/op-node/rollup/sync"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
)

type Config struct {
	L1 L1EndpointSetup
	L2 L2EndpointSetup

	Beacon L1BeaconEndpointSetup

	InteropConfig interop.Setup

	Driver driver.Config

	Rollup rollup.Config

	DependencySet depset.DependencySet

	// P2PSigner will be used for signing off on published content
	// if the node is sequencing and if the p2p stack is enabled
	P2PSigner p2p.SignerSetup

	RPC oprpc.CLIConfig

	P2P p2p.SetupP2P

	Metrics opmetrics.CLIConfig

	Pprof oppprof.CLIConfig

	// Used to poll the L1 for new finalized or safe blocks
	L1EpochPollInterval time.Duration

	ConfigPersistence ConfigPersistence

	// Path to store safe head database. Disabled when set to empty string
	SafeDBPath string

	// RuntimeConfigReloadInterval defines the interval between runtime config reloads.
	// Disabled if <= 0.
	// Runtime config changes should be picked up from log-events,
	// but if log-events are not coming in (e.g. not syncing blocks) then the reload ensures the config stays accurate.
	RuntimeConfigReloadInterval time.Duration

	// Optional
	Tracer tracer.Tracer

	Sync sync.Config

	// To halt when detecting the node does not support a signaled protocol version
	// change of the given severity (major/minor/patch). Disabled if empty.
	RollupHalt string

	// Cancel to request a premature shutdown of the node itself, e.g. when halting. This may be nil.
	Cancel context.CancelCauseFunc

	// Conductor is used to determine this node is the leader sequencer.
	ConductorEnabled    bool
	ConductorRpc        ConductorRPCFunc
	ConductorRpcTimeout time.Duration

	// AltDA config
	AltDA altda.CLIConfig

	IgnoreMissingPectraBlobSchedule bool
	FetchWithdrawalRootFromState    bool

	// Experimental. Enables new opstack RPC namespace. Used by op-test-sequencer.
	ExperimentalOPStackAPI bool
}

// ConductorRPCFunc retrieves the endpoint. The RPC may not immediately be available.
type ConductorRPCFunc func(ctx context.Context) (string, error)

func (cfg *Config) LoadPersisted(log log.Logger) error {
	if !cfg.Driver.SequencerEnabled {
		return nil
	}
	if state, err := cfg.ConfigPersistence.SequencerState(); err != nil {
		return err
	} else if state != StateUnset {
		stopped := state == StateStopped
		if stopped != cfg.Driver.SequencerStopped {
			log.Warn(fmt.Sprintf("Overriding %v with persisted state", flags.SequencerStoppedFlag.Name), "stopped", stopped)
		}
		cfg.Driver.SequencerStopped = stopped
	} else {
		log.Info("No persisted sequencer state loaded")
	}
	return nil
}

var ErrMissingPectraBlobSchedule = errors.New("probably missing Pectra blob schedule")

// Check verifies that the given configuration makes sense
func (cfg *Config) Check() error {
	if err := cfg.L1.Check(); err != nil {
		return fmt.Errorf("l1 endpoint config error: %w", err)
	}
	if err := cfg.L2.Check(); err != nil {
		return fmt.Errorf("l2 endpoint config error: %w", err)
	}
	if cfg.Rollup.EcotoneTime != nil {
		if cfg.Beacon == nil {
			return fmt.Errorf("the Ecotone upgrade is scheduled (timestamp = %d) but no L1 Beacon API endpoint is configured", *cfg.Rollup.EcotoneTime)
		}
		if err := cfg.Beacon.Check(); err != nil {
			return fmt.Errorf("misconfigured L1 Beacon API endpoint: %w", err)
		}
	}
	if cfg.InteropConfig == nil {
		return errors.New("missing interop config")
	}
	if err := cfg.InteropConfig.Check(); err != nil {
		return fmt.Errorf("misconfigured interop: %w", err)
	}
	if err := cfg.Rollup.Check(); err != nil {
		return fmt.Errorf("rollup config error: %w", err)
	}
	if !cfg.IgnoreMissingPectraBlobSchedule && cfg.Rollup.ProbablyMissingPectraBlobSchedule() {
		log.Error("Your rollup config seems to be missing the Pectra blob schedule fix. " +
			"Reach out to your chain operator for the correct Pectra blob schedule configuration. " +
			"If you know what you are doing, you can disable this error by setting the " +
			"'--ignore-missing-pectra-blob-schedule' flag or 'IGNORE_MISSING_PECTRA_BLOB_SCHEDULE' env var.")
		return ErrMissingPectraBlobSchedule
	}
	if cfg.Rollup.InteropTime != nil && cfg.DependencySet == nil {
		return fmt.Errorf("the Interop upgrade is scheduled (timestamp = %d) but not dependency set is configured", *cfg.Rollup.InteropTime)
	}
	if err := cfg.Metrics.Check(); err != nil {
		return fmt.Errorf("metrics config error: %w", err)
	}
	if err := cfg.Pprof.Check(); err != nil {
		return fmt.Errorf("pprof config error: %w", err)
	}
	if cfg.P2P != nil {
		if err := cfg.P2P.Check(); err != nil {
			return fmt.Errorf("p2p config error: %w", err)
		}
	}
	if !(cfg.RollupHalt == "" || cfg.RollupHalt == "major" || cfg.RollupHalt == "minor" || cfg.RollupHalt == "patch") {
		return fmt.Errorf("invalid rollup halting option: %q", cfg.RollupHalt)
	}
	if cfg.ConductorEnabled {
		if state, _ := cfg.ConfigPersistence.SequencerState(); state != StateUnset {
			return fmt.Errorf("config persistence must be disabled when conductor is enabled")
		}
		if !cfg.Driver.SequencerEnabled {
			return fmt.Errorf("sequencer must be enabled when conductor is enabled")
		}
	}
	if err := cfg.AltDA.Check(); err != nil {
		return fmt.Errorf("altDA config error: %w", err)
	}
	if cfg.AltDA.Enabled {
		log.Warn("Alt-DA Mode is a Beta feature of the MIT licensed OP Stack.  While it has received initial review from core contributors, it is still undergoing testing, and may have bugs or other issues.")
	}
	return nil
}

func (cfg *Config) P2PEnabled() bool {
	return cfg.P2P != nil && !cfg.P2P.Disabled()
}
