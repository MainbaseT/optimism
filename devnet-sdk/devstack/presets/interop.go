package presets

import (
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/devtest"
	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/dsl"
	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/shim"
	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/stack"
	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/stack/match"
	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/sysgo"
)

type SimpleInterop struct {
	Log          log.Logger
	T            devtest.T
	Supervisor   *dsl.Supervisor
	ControlPlane stack.ControlPlane

	L2ChainA *dsl.L2Network
	L2ChainB *dsl.L2Network
}

func NewSimpleInterop(dest *TestSetup[*SimpleInterop]) stack.Option {
	return func(orch stack.Orchestrator) {
		if _, isSysGo := orch.(*sysgo.Orchestrator); isSysGo {
			startInProcessSimpleInterop(orch)
		}
		*dest = func(t devtest.T) *SimpleInterop {
			return hydrateSimpleInterop(t, orch)
		}
	}
}

// startInProcessSimpleInterop starts a new system that meets the simple interop criteria
func startInProcessSimpleInterop(orch stack.Orchestrator) {
	var ids sysgo.DefaultInteropSystemIDs
	opt := sysgo.DefaultInteropSystem(&ids)
	opt(orch)
}

// hydrateSimpleInterop hydrates the test specific view of a shared system and selects the resources required for
// a simple interop system.
func hydrateSimpleInterop(t devtest.T, orch stack.Orchestrator) *SimpleInterop {
	system := shim.NewSystem(t)
	orch.Hydrate(system)

	t.Require().GreaterOrEqual(len(system.Supervisors()), 1, "expected at least one supervisor")
	// At this point, any supervisor is acceptable but as the DSL gets fleshed out this should be selecting supervisors
	// that fit with specific networks and nodes. That will likely require expanding the metadata exposed by the system
	// since currently there's no way to tell which nodes are using which supervisor.
	supervisorId := system.SupervisorIDs()[0]
	return &SimpleInterop{
		Log:          t.Logger(),
		T:            t,
		Supervisor:   dsl.NewSupervisor(system.Supervisor(supervisorId)),
		ControlPlane: orch.ControlPlane(),
		L2ChainA:     dsl.NewL2Network(system.L2Network(match.L2ChainA)),
		L2ChainB:     dsl.NewL2Network(system.L2Network(match.L2ChainB)),
	}
}
