package driver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/engine"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
)

var (
	errTestReset = errors.New("reset test err")
	errTestTemp  = errors.New("temp test err")
	errTestCrit  = errors.New("crit test err")
)

func TestProgramDeriver(t *testing.T) {
	newProgram := func(t *testing.T, target uint64) (*ProgramDeriver, *testutils.MockEmitter) {
		m := &testutils.MockEmitter{}
		logger := testlog.Logger(t, log.LevelInfo)
		prog := &ProgramDeriver{
			logger:         logger,
			Emitter:        m,
			targetBlockNum: target,
		}
		return prog, m
	}
	// step 0 assumption: engine performs reset upon ResetEngineRequestEvent.
	// step 1: engine completes reset
	t.Run("engine reset confirmed", func(t *testing.T) {
		p, m := newProgram(t, 1000)
		m.ExpectOnce(derive.ConfirmPipelineResetEvent{})
		m.ExpectOnce(engine.PendingSafeRequestEvent{})
		p.OnEvent(context.Background(), engine.EngineResetConfirmedEvent{})
		m.AssertExpectations(t)
		require.False(t, p.closing)
		require.NoError(t, p.resultError)
		require.False(t, p.closing)
		require.NoError(t, p.resultError)
	})
	// step 2: more derivation work, triggered when pending safe data is published
	t.Run("pending safe update", func(t *testing.T) {
		p, m := newProgram(t, 1000)
		ref := eth.L2BlockRef{Number: 123}
		m.ExpectOnce(derive.PipelineStepEvent{PendingSafe: ref})
		p.OnEvent(context.Background(), engine.PendingSafeUpdateEvent{PendingSafe: ref})
		m.AssertExpectations(t)
		require.False(t, p.closing)
		require.NoError(t, p.resultError)
	})
	// step 3: if no attributes are generated, loop back to derive more.
	t.Run("deriver more", func(t *testing.T) {
		p, m := newProgram(t, 1000)
		m.ExpectOnce(engine.PendingSafeRequestEvent{})
		p.OnEvent(context.Background(), derive.DeriverMoreEvent{})
		m.AssertExpectations(t)
		require.False(t, p.closing)
		require.NoError(t, p.resultError)
	})
	// step 4: if attributes are derived, pass them to the engine.
	t.Run("derived attributes", func(t *testing.T) {
		p, m := newProgram(t, 1000)
		attrib := &derive.AttributesWithParent{Parent: eth.L2BlockRef{Number: 123}}
		m.ExpectOnce(derive.ConfirmReceivedAttributesEvent{})
		m.ExpectOnce(engine.BuildStartEvent{Attributes: attrib})
		p.OnEvent(context.Background(), derive.DerivedAttributesEvent{Attributes: attrib})
		m.AssertExpectations(t)
		require.False(t, p.closing)
		require.NoError(t, p.resultError)
	})
	// step 5: if attributes were invalid, continue with derivation for new attributes.
	t.Run("invalid payload", func(t *testing.T) {
		p, m := newProgram(t, 1000)
		m.ExpectOnce(engine.PendingSafeRequestEvent{})
		p.OnEvent(context.Background(), engine.InvalidPayloadAttributesEvent{Attributes: &derive.AttributesWithParent{}})
		m.AssertExpectations(t)
		require.False(t, p.closing)
		require.NoError(t, p.resultError)
	})
	// step 6: if attributes were valid, we may have reached the target.
	// Or back to step 2 (PendingSafeUpdateEvent)
	t.Run("forkchoice update", func(t *testing.T) {
		t.Run("surpassed", func(t *testing.T) {
			p, m := newProgram(t, 42)
			p.OnEvent(context.Background(), engine.ForkchoiceUpdateEvent{SafeL2Head: eth.L2BlockRef{Number: 42 + 1}})
			m.AssertExpectations(t)
			require.True(t, p.closing)
			require.NoError(t, p.resultError)
		})
		t.Run("completed", func(t *testing.T) {
			p, m := newProgram(t, 42)
			p.OnEvent(context.Background(), engine.ForkchoiceUpdateEvent{SafeL2Head: eth.L2BlockRef{Number: 42}})
			m.AssertExpectations(t)
			require.True(t, p.closing)
			require.NoError(t, p.resultError)
		})
		t.Run("incomplete", func(t *testing.T) {
			p, m := newProgram(t, 42)
			p.OnEvent(context.Background(), engine.ForkchoiceUpdateEvent{SafeL2Head: eth.L2BlockRef{Number: 42 - 1}})
			m.AssertExpectations(t)
			require.False(t, p.closing)
			require.NoError(t, p.resultError)
		})
	})
	// Do not stop processing when the deriver is idle, the engine may still be busy and create further events.
	t.Run("deriver idle", func(t *testing.T) {
		p, m := newProgram(t, 1000)
		p.OnEvent(context.Background(), derive.DeriverIdleEvent{})
		m.AssertExpectations(t)
		require.False(t, p.closing)
		require.NoError(t, p.resultError)
	})
	// on inconsistent chain data: stop with error
	t.Run("reset event", func(t *testing.T) {
		p, m := newProgram(t, 1000)
		p.OnEvent(context.Background(), rollup.ResetEvent{Err: errTestReset})
		m.AssertExpectations(t)
		require.True(t, p.closing)
		require.Error(t, p.resultError)
	})
	// on L1 temporary error: stop with error
	t.Run("L1 temporary error event", func(t *testing.T) {
		p, m := newProgram(t, 1000)
		p.OnEvent(context.Background(), rollup.L1TemporaryErrorEvent{Err: errTestTemp})
		m.AssertExpectations(t)
		require.True(t, p.closing)
		require.Error(t, p.resultError)
	})
	// on engine temporary error: continue derivation (because legacy, not all connection related)
	t.Run("engine temp error event", func(t *testing.T) {
		p, m := newProgram(t, 1000)
		m.ExpectOnce(engine.PendingSafeRequestEvent{})
		p.OnEvent(context.Background(), rollup.EngineTemporaryErrorEvent{Err: errTestTemp})
		m.AssertExpectations(t)
		require.False(t, p.closing)
		require.NoError(t, p.resultError)
	})
	// on critical error: stop
	t.Run("critical error event", func(t *testing.T) {
		p, m := newProgram(t, 1000)
		p.OnEvent(context.Background(), rollup.ResetEvent{Err: errTestCrit})
		m.AssertExpectations(t)
		require.True(t, p.closing)
		require.Error(t, p.resultError)
	})
	t.Run("unknown event", func(t *testing.T) {
		p, m := newProgram(t, 1000)
		p.OnEvent(context.Background(), TestEvent{})
		m.AssertExpectations(t)
		require.False(t, p.closing)
		require.NoError(t, p.resultError)
	})
}

type TestEvent struct {
}

func (ev TestEvent) String() string {
	return "test-event"
}
