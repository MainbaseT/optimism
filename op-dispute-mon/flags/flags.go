package flags

import (
	"fmt"
	"strings"

	challengerFlags "github.com/ethereum-optimism/optimism/op-challenger/flags"
	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	"github.com/ethereum-optimism/optimism/op-service/flags"
	"github.com/urfave/cli/v2"

	"github.com/ethereum-optimism/optimism/op-dispute-mon/config"
	opservice "github.com/ethereum-optimism/optimism/op-service"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	"github.com/ethereum/go-ethereum/common"
)

const (
	envVarPrefix = "OP_DISPUTE_MON"
)

func prefixEnvVars(name string) []string {
	return opservice.PrefixEnvVar(envVarPrefix, name)
}

var (
	// Required Flags
	L1EthRpcFlag = &cli.StringFlag{
		Name:    "l1-eth-rpc",
		Usage:   "HTTP provider URL for L1.",
		EnvVars: prefixEnvVars("L1_ETH_RPC"),
	}
	// Optional Flags
	RollupRpcFlag = &cli.StringSliceFlag{
		Name:    "rollup-rpc",
		Usage:   "HTTP provider URL for the rollup node. Multiple URLs can be specified for redundancy.",
		EnvVars: prefixEnvVars("ROLLUP_RPC"),
	}
	SupervisorRpcFlag = &cli.StringSliceFlag{
		Name:    "supervisor-rpc",
		Usage:   "HTTP provider URL for supervisor nodes. Multiple URLs can be specified for redundancy.",
		EnvVars: prefixEnvVars("SUPERVISOR_RPC"),
	}
	GameFactoryAddressFlag = &cli.StringFlag{
		Name:    "game-factory-address",
		Usage:   "Address of the fault game factory contract.",
		EnvVars: prefixEnvVars("GAME_FACTORY_ADDRESS"),
	}
	NetworkFlag = &cli.StringSliceFlag{
		Name:    flags.NetworkFlagName,
		Usage:   fmt.Sprintf("Predefined network selection. Available networks: %s", strings.Join(chaincfg.AvailableNetworks(), ", ")),
		EnvVars: prefixEnvVars("NETWORK"),
	}
	HonestActorsFlag = &cli.StringSliceFlag{
		Name:    "honest-actors",
		Usage:   "List of honest actors that are monitored for any claims that are resolved against them.",
		EnvVars: prefixEnvVars("HONEST_ACTORS"),
	}
	MonitorIntervalFlag = &cli.DurationFlag{
		Name:    "monitor-interval",
		Usage:   "The interval at which the dispute monitor will check for new games to monitor.",
		EnvVars: prefixEnvVars("MONITOR_INTERVAL"),
		Value:   config.DefaultMonitorInterval,
	}
	GameWindowFlag = &cli.DurationFlag{
		Name: "game-window",
		Usage: "The time window which the monitor will consider games to report on. " +
			"This should include a bond claim buffer for games outside the maximum game duration.",
		EnvVars: prefixEnvVars("GAME_WINDOW"),
		Value:   config.DefaultGameWindow,
	}
	IgnoredGamesFlag = &cli.StringSliceFlag{
		Name:    "ignored-games",
		Usage:   "List of game addresses to exclude from monitoring.",
		EnvVars: prefixEnvVars("IGNORED_GAMES"),
	}
	MaxConcurrencyFlag = &cli.UintFlag{
		Name:    "max-concurrency",
		Usage:   "Maximum number of threads to use when fetching game data",
		EnvVars: prefixEnvVars("MAX_CONCURRENCY"),
		Value:   config.DefaultMaxConcurrency,
	}
)

// requiredFlags are checked by [CheckRequired]
var requiredFlags = []cli.Flag{
	L1EthRpcFlag,
}

// optionalFlags is a list of unchecked cli flags
var optionalFlags = []cli.Flag{
	RollupRpcFlag,
	SupervisorRpcFlag,
	GameFactoryAddressFlag,
	NetworkFlag,
	HonestActorsFlag,
	MonitorIntervalFlag,
	GameWindowFlag,
	IgnoredGamesFlag,
	MaxConcurrencyFlag,
}

func init() {
	optionalFlags = append(optionalFlags, oplog.CLIFlags(envVarPrefix)...)
	optionalFlags = append(optionalFlags, opmetrics.CLIFlags(envVarPrefix)...)
	optionalFlags = append(optionalFlags, oppprof.CLIFlags(envVarPrefix)...)

	Flags = append(requiredFlags, optionalFlags...)
}

// Flags contains the list of configuration options available to the binary.
var Flags []cli.Flag

func CheckRequired(ctx *cli.Context) error {
	for _, f := range requiredFlags {
		if !ctx.IsSet(f.Names()[0]) {
			return fmt.Errorf("flag %s is required", f.Names()[0])
		}
	}
	if len(ctx.StringSlice(RollupRpcFlag.Name)) == 0 && len(ctx.StringSlice(SupervisorRpcFlag.Name)) == 0 {
		return fmt.Errorf("flag %s or %s is required", RollupRpcFlag.Name, SupervisorRpcFlag.Name)
	}
	return nil
}

// NewConfigFromCLI parses the Config from the provided flags or environment variables.
func NewConfigFromCLI(ctx *cli.Context) (*config.Config, error) {
	if err := CheckRequired(ctx); err != nil {
		return nil, err
	}
	gameFactoryAddress, err := challengerFlags.FactoryAddress(ctx)
	if err != nil {
		return nil, err
	}

	var actors []common.Address
	if ctx.IsSet(HonestActorsFlag.Name) {
		for _, addrStr := range ctx.StringSlice(HonestActorsFlag.Name) {
			actor, err := opservice.ParseAddress(addrStr)
			if err != nil {
				return nil, fmt.Errorf("invalid honest actor address: %w", err)
			}
			actors = append(actors, actor)
		}
	}

	var ignoredGames []common.Address
	if ctx.IsSet(IgnoredGamesFlag.Name) {
		for _, addrStr := range ctx.StringSlice(IgnoredGamesFlag.Name) {
			game, err := opservice.ParseAddress(addrStr)
			if err != nil {
				return nil, fmt.Errorf("invalid ignored game address: %w", err)
			}
			ignoredGames = append(ignoredGames, game)
		}
	}

	maxConcurrency := ctx.Uint(MaxConcurrencyFlag.Name)
	if maxConcurrency == 0 {
		return nil, fmt.Errorf("%v must not be 0", MaxConcurrencyFlag.Name)
	}

	metricsConfig := opmetrics.ReadCLIConfig(ctx)
	pprofConfig := oppprof.ReadCLIConfig(ctx)

	return &config.Config{
		L1EthRpc:           ctx.String(L1EthRpcFlag.Name),
		GameFactoryAddress: gameFactoryAddress,
		RollupRpcs:         ctx.StringSlice(RollupRpcFlag.Name),
		SupervisorRpcs:     ctx.StringSlice(SupervisorRpcFlag.Name),

		HonestActors:    actors,
		MonitorInterval: ctx.Duration(MonitorIntervalFlag.Name),
		GameWindow:      ctx.Duration(GameWindowFlag.Name),
		IgnoredGames:    ignoredGames,
		MaxConcurrency:  maxConcurrency,

		MetricsConfig: metricsConfig,
		PprofConfig:   pprofConfig,
	}, nil
}
