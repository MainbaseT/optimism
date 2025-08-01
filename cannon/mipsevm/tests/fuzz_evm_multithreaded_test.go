package tests

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/exec"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/multithreaded"
	mtutil "github.com/ethereum-optimism/optimism/cannon/mipsevm/multithreaded/testutil"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/register"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/testutil"
)

func FuzzStateSyscallCloneMT(f *testing.F) {
	versions := GetMipsVersionTestCases(f)
	require.NotZero(f, len(versions), "must have at least one multithreaded version supported")
	f.Fuzz(func(t *testing.T, nextThreadId, stackPtr Word, seed int64, version uint) {
		v := versions[int(version)%len(versions)]
		goVm := v.VMFactory(nil, os.Stdout, os.Stderr, testutil.CreateLogger(), mtutil.WithRandomization(seed))
		state := mtutil.GetMtState(t, goVm)
		// Update existing threads to avoid collision with nextThreadId
		if mtutil.FindThread(state, nextThreadId) != nil {
			for i, t := range mtutil.GetAllThreads(state) {
				t.ThreadId = nextThreadId - Word(i+1)
			}
		}

		// Setup
		state.NextThreadId = nextThreadId
		testutil.StoreInstruction(state.GetMemory(), state.GetPC(), syscallInsn)
		state.GetRegistersRef()[2] = arch.SysClone
		state.GetRegistersRef()[4] = exec.ValidCloneFlags
		state.GetRegistersRef()[5] = stackPtr
		step := state.GetStep()

		// Set up expectations
		expected := mtutil.NewExpectedState(t, state)
		expected.Step += 1
		// Set original thread expectations
		expected.PrestateActiveThread().PC = state.GetCpu().NextPC
		expected.PrestateActiveThread().NextPC = state.GetCpu().NextPC + 4
		expected.PrestateActiveThread().Registers[2] = nextThreadId
		expected.PrestateActiveThread().Registers[7] = 0
		// Set expectations for new, cloned thread
		expectedNewThread := expected.ExpectNewThread()
		expectedNewThread.PC = state.GetCpu().NextPC
		expectedNewThread.NextPC = state.GetCpu().NextPC + 4
		expectedNewThread.Registers[register.RegSyscallNum] = 0
		expectedNewThread.Registers[register.RegSyscallErrno] = 0
		expectedNewThread.Registers[register.RegSP] = stackPtr
		expected.ExpectActiveThreadId(nextThreadId)
		expected.ExpectNextThreadId(nextThreadId + 1)
		expected.ExpectContextSwitch()

		stepWitness, err := goVm.Step(true)
		require.NoError(t, err)
		require.False(t, stepWitness.HasPreimage())

		expected.Validate(t, state)
		testutil.ValidateEVM(t, stepWitness, step, goVm, multithreaded.GetStateHashFn(), v.Contracts)
	})
}
