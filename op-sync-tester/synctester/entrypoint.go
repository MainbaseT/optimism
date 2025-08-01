package synctester

import (
	"context"
	"fmt"

	opservice "github.com/ethereum-optimism/optimism/op-service"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"

	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	"github.com/ethereum-optimism/optimism/op-sync-tester/config"
	"github.com/ethereum-optimism/optimism/op-sync-tester/flags"
	"github.com/ethereum/go-ethereum/log"
	"github.com/urfave/cli/v2"
)

type MainFn func(ctx context.Context, cfg *config.Config, logger log.Logger) (cliapp.Lifecycle, error)

// Main is the entrypoint into the service.
// This method returns a cliapp.LifecycleAction, to create an op-service CLI-lifecycle-managed service with.
func Main(version string, fn MainFn) cliapp.LifecycleAction {
	return func(cliCtx *cli.Context, closeApp context.CancelCauseFunc) (cliapp.Lifecycle, error) {
		if err := flags.CheckRequired(cliCtx); err != nil {
			return nil, err
		}
		cfg := flags.ConfigFromCLI(cliCtx, version)
		if err := cfg.Check(); err != nil {
			return nil, fmt.Errorf("invalid CLI flags: %w", err)
		}

		l := oplog.NewLogger(oplog.AppOut(cliCtx), cfg.LogConfig)
		oplog.SetGlobalLogHandler(l.Handler())
		opservice.ValidateEnvVars(flags.EnvVarPrefix, flags.Flags, l)

		l.Info("Initializing op-sync-tester")
		return fn(cliCtx.Context, cfg, l)
	}
}
