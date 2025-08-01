package driver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/event"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

var mockErr = errors.New("mock error")

type fakeEnd struct {
	closing bool
	result  error
}

func (d *fakeEnd) Closing() bool {
	return d.closing
}

func (d *fakeEnd) Result() (eth.L2BlockRef, error) {
	return eth.L2BlockRef{}, d.result
}

func TestDriver(t *testing.T) {
	newTestDriver := func(t *testing.T, onEvent func(d *Driver, end *fakeEnd, ev event.Event)) *Driver {
		logger := testlog.Logger(t, log.LevelInfo)
		end := &fakeEnd{}
		d := &Driver{
			logger: logger,
			end:    end,
		}
		d.deriver = event.DeriverFunc(func(ctx context.Context, ev event.Event) bool {
			onEvent(d, end, ev)
			return true
		})
		return d
	}

	t.Run("insta complete", func(t *testing.T) {
		d := newTestDriver(t, func(d *Driver, end *fakeEnd, ev event.Event) {
			end.closing = true
		})
		_, err := d.RunComplete()
		require.NoError(t, err)
	})

	t.Run("insta error", func(t *testing.T) {
		d := newTestDriver(t, func(d *Driver, end *fakeEnd, ev event.Event) {
			end.closing = true
			end.result = mockErr
		})
		_, err := d.RunComplete()
		require.ErrorIs(t, mockErr, err)
	})

	t.Run("success after a few events", func(t *testing.T) {
		count := 0
		d := newTestDriver(t, func(d *Driver, end *fakeEnd, ev event.Event) {
			if count > 3 {
				end.closing = true
				return
			}
			count += 1
			d.Emit(context.Background(), TestEvent{})
		})
		_, err := d.RunComplete()
		require.NoError(t, err)
	})

	t.Run("error after a few events", func(t *testing.T) {
		count := 0
		d := newTestDriver(t, func(d *Driver, end *fakeEnd, ev event.Event) {
			if count > 3 {
				end.closing = true
				end.result = mockErr
				return
			}
			count += 1
			d.Emit(context.Background(), TestEvent{})
		})
		_, err := d.RunComplete()
		require.ErrorIs(t, mockErr, err)
	})

	t.Run("exhaust events", func(t *testing.T) {
		count := 0
		d := newTestDriver(t, func(d *Driver, end *fakeEnd, ev event.Event) {
			if count < 3 { // stop generating events after a while, without changing end condition
				d.Emit(context.Background(), TestEvent{})
			}
			count += 1
		})
		// No further processing to be done so evaluate if the claims output root is correct.
		_, err := d.RunComplete()
		require.NoError(t, err)
	})

	t.Run("queued events", func(t *testing.T) {
		count := 0
		d := newTestDriver(t, func(d *Driver, end *fakeEnd, ev event.Event) {
			if count < 3 {
				d.Emit(context.Background(), TestEvent{})
				d.Emit(context.Background(), TestEvent{})
			}
			count += 1
		})
		_, err := d.RunComplete()
		require.NoError(t, err)
		// add 1 for initial event that RunComplete fires
		require.Equal(t, 1+3*2, count, "must have queued up 2 events 3 times")
	})
}
