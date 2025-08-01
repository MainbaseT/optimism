package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/standard"

	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum-optimism/optimism/op-validator/pkg/service"
	"github.com/urfave/cli/v2"
)

const EnvVarPrefix = "OP_VALIDATOR"

var (
	GitCommit = ""
	GitDate   = ""
	Version   = ""
)

func main() {
	app := cli.NewApp()
	app.Version = Version
	app.Name = "op-validator"
	app.Usage = "Optimism Validator Service"
	app.Description = "CLI to validate Optimism L2 deployments"
	app.Flags = oplog.CLIFlags(EnvVarPrefix)
	app.Commands = []*cli.Command{
		{
			Name:  "validate",
			Usage: "Run validation for a specific version",
			Subcommands: []*cli.Command{
				versionCmd(standard.ContractsV180Tag),
				versionCmd(standard.ContractsV200Tag),
				versionCmd(standard.ContractsV300Tag),
				versionCmd(standard.ContractsV400Tag),
			},
		},
	}
	app.Writer = os.Stdout
	app.ErrWriter = os.Stderr

	err := app.Run(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Application failed: %v\n", err)
		os.Exit(1)
	}
}

func versionCmd(version string) *cli.Command {
	return &cli.Command{
		Name:  strings.Replace(version, "op-contracts/", "", 1),
		Usage: fmt.Sprintf("Run validation for %s", version),
		Flags: append(service.ValidateFlags, oplog.CLIFlags(EnvVarPrefix)...),
		Action: func(cliCtx *cli.Context) error {
			return service.ValidateCmd(cliCtx, version)
		},
	}
}
