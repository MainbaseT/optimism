package driver

import (
	"context"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/engine"
	"github.com/ethereum-optimism/optimism/op-service/event"
)

// ProgramDeriver expresses how engine and derivation events are
// translated and monitored to execute the pure L1 to L2 state transition.
//
// The ProgramDeriver stops at the target block number or with an error result.
type ProgramDeriver struct {
	logger log.Logger

	Emitter event.Emitter

	closing        bool
	result         eth.L2BlockRef
	resultError    error
	targetBlockNum uint64
}

func (d *ProgramDeriver) Closing() bool {
	return d.closing
}

func (d *ProgramDeriver) Result() (eth.L2BlockRef, error) {
	return d.result, d.resultError
}

func (d *ProgramDeriver) OnEvent(ctx context.Context, ev event.Event) bool {
	switch x := ev.(type) {
	case engine.EngineResetConfirmedEvent:
		d.Emitter.Emit(ctx, derive.ConfirmPipelineResetEvent{})
		// After initial reset we can request the pending-safe block,
		// where attributes will be generated on top of.
		d.Emitter.Emit(ctx, engine.PendingSafeRequestEvent{})
	case engine.PendingSafeUpdateEvent:
		d.Emitter.Emit(ctx, derive.PipelineStepEvent{PendingSafe: x.PendingSafe})
	case derive.DeriverMoreEvent:
		d.Emitter.Emit(ctx, engine.PendingSafeRequestEvent{})
	case derive.DerivedAttributesEvent:
		// Allow new attributes to be generated.
		// We will process the current attributes synchronously,
		// triggering a single PendingSafeUpdateEvent or InvalidPayloadAttributesEvent,
		// to continue derivation from.
		d.Emitter.Emit(ctx, derive.ConfirmReceivedAttributesEvent{})
		// No need to queue the attributes, since there is no unsafe chain to consolidate against,
		// and no temporary-error retry to perform on block processing.
		d.Emitter.Emit(ctx, engine.BuildStartEvent{Attributes: x.Attributes})
	case engine.InvalidPayloadAttributesEvent:
		// If a set of attributes was invalid, then we drop the attributes,
		// and continue with the next.
		d.Emitter.Emit(ctx, engine.PendingSafeRequestEvent{})
	case engine.ForkchoiceUpdateEvent:
		// Track latest head.
		if x.SafeL2Head.Number >= d.result.Number {
			d.result = x.SafeL2Head
		}
		// Stop if we have reached the target block
		if x.SafeL2Head.Number >= d.targetBlockNum {
			d.logger.Info("Derivation complete: reached L2 block as safe", "head", x.SafeL2Head)
			d.closing = true
		}
	case engine.LocalSafeUpdateEvent:
		// Track latest head.
		if x.Ref.Number >= d.result.Number {
			d.result = x.Ref
		}
		// Stop if we have reached the target block
		if x.Ref.Number >= d.targetBlockNum {
			d.logger.Info("Derivation complete: reached L2 block as local safe", "head", x.Ref)
			d.closing = true
		}
	case derive.DeriverIdleEvent:
		// We don't close the deriver yet, as the engine may still be processing events to reach
		// the target. A ForkchoiceUpdateEvent will close the deriver when the target is reached.
		d.logger.Info("Derivation complete: no further L1 data to process")
	case rollup.ResetEvent:
		d.closing = true
		d.resultError = fmt.Errorf("unexpected reset error: %w", x.Err)
	case rollup.L1TemporaryErrorEvent:
		d.closing = true
		d.resultError = fmt.Errorf("unexpected L1 error: %w", x.Err)
	case rollup.EngineTemporaryErrorEvent:
		// (Legacy case): While most temporary errors are due to requests for external data failing which can't happen,
		// they may also be returned due to other events like channels timing out so need to be handled
		d.logger.Warn("Temporary error in derivation", "err", x.Err)
		d.Emitter.Emit(ctx, engine.PendingSafeRequestEvent{})
	case rollup.CriticalErrorEvent:
		d.closing = true
		d.resultError = x.Err
	default:
		// Other events can be ignored safely.
		// They are broadcast, but only consumed by the other derivers,
		// or do not affect the state-transition.
		return false
	}
	return true
}
