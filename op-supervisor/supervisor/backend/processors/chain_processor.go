package processors

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/event"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/superevents"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type Source interface {
	L2BlockRefByNumber(ctx context.Context, number uint64) (eth.L2BlockRef, error)
	FetchReceipts(ctx context.Context, blockHash common.Hash) (gethtypes.Receipts, error)
}

type LogProcessor interface {
	ProcessLogs(ctx context.Context, block eth.BlockRef, receipts gethtypes.Receipts) error
}

type DatabaseRewinder interface {
	Rewind(chain eth.ChainID, headBlock eth.BlockID) error
	LatestBlockNum(chain eth.ChainID) (num uint64, ok bool)
	AcceptedBlock(chainID eth.ChainID, id eth.BlockID) error
}

type BlockProcessorFn func(ctx context.Context, block eth.BlockRef) error

func (fn BlockProcessorFn) ProcessBlock(ctx context.Context, block eth.BlockRef) error {
	return fn(ctx, block)
}

// ChainProcessor is a HeadProcessor that fills in any skipped blocks between head update events.
// It ensures that, absent reorgs, every block in the chain is processed even if some head advancements are skipped.
type ChainProcessor struct {
	log log.Logger

	clients      []Source
	activeClient Source
	clientIndex  int
	clientsTried int
	clientLock   sync.Mutex

	chain eth.ChainID

	running atomic.Bool
	target  uint64

	systemContext context.Context

	processor LogProcessor
	rewinder  DatabaseRewinder

	emitter event.Emitter

	maxFetcherThreads int
}

var (
	_ event.AttachEmitter = (*ChainProcessor)(nil)
	_ event.Deriver       = (*ChainProcessor)(nil)
)

func NewChainProcessor(systemContext context.Context, log log.Logger, chain eth.ChainID, processor LogProcessor, rewinder DatabaseRewinder) *ChainProcessor {
	out := &ChainProcessor{
		systemContext:     systemContext,
		log:               log.New("chain", chain),
		chain:             chain,
		processor:         processor,
		rewinder:          rewinder,
		maxFetcherThreads: 10,
	}
	return out
}

func (s *ChainProcessor) AttachEmitter(em event.Emitter) {
	s.emitter = em
}

func (s *ChainProcessor) AddSource(cl Source) {
	s.clientLock.Lock()
	defer s.clientLock.Unlock()
	s.clients = append(s.clients, cl)
	if s.activeClient == nil {
		s.activeClient = s.clients[0]
	}
}

// nextNum returns the next block number to process.
// It returns 0 if the rewinder is empty, so there's no start to process from.
func (s *ChainProcessor) nextNum() uint64 {
	headNum, ok := s.rewinder.LatestBlockNum(s.chain)
	if !ok {
		return 0
	}
	return headNum + 1
}

func (s *ChainProcessor) ProcessChain(target uint64) {
	s.UpdateTarget(target)
	if s.running.CompareAndSwap(false, true) {
		s.index()
	}
}

func (s *ChainProcessor) OnEvent(ctx context.Context, ev event.Event) bool {
	switch x := ev.(type) {
	case superevents.ChainIndexingContinueEvent:
		if x.ChainID != s.chain {
			return false
		}
		// always continue indexing when a continue event is received
		// continue events only come from the index function
		s.index()
	default:
		return false
	}
	return true
}

func (s *ChainProcessor) UpdateTarget(newTarget uint64) {
	if newTarget < s.target {
		s.log.Debug("Target is already higher than update", "newTarget", newTarget, "oldTarget", s.target)
		return
	}
	s.target = newTarget
}

// index is the main processing loop. It triggers a r
func (s *ChainProcessor) index() {
	// evaluate if indexing is needed
	target := s.target
	next := s.nextNum()
	if next == 0 {
		s.log.Warn("Dropping processing request, DB empty, need activation block first", "target", target)
		s.running.Store(false)
		return
	} else if target < next {
		s.log.Debug("Indexing for target block already done", "target", target, "next", s.nextNum())
		s.running.Store(false)
		return
	}
	// index the blocks up to the target
	processed, err := s.rangeUpdate(target)
	if err != nil {
		if errors.Is(err, ethereum.NotFound) {
			s.log.Debug("indexer cannot find next block yet", "target", target, "err", err)
		} else if errors.Is(err, types.ErrNoRPCSource) {
			s.log.Warn("No RPC source configured, cannot process new blocks")
		} else {
			s.log.Error("Failed to index blocks", "err", err)
		}
	}
	// if the client failed to get *any* blocks, it probably isn't the source of this sync request
	// so we should try the next client. Clients will be tried in round-robin order until one succeeds.
	// or until they've all been tried, at which point the indexer will idle.
	if processed == 0 {
		if s.clientsTried < len(s.clients) {
			s.log.Debug("Active client found no blocks, trying again with next client", "activeClient", s.activeClient)
			s.nextActiveClient()
			s.emitter.Emit(s.systemContext, superevents.ChainIndexingContinueEvent{
				ChainID: s.chain,
			})
			return
		} else {
			s.log.Debug("All clients failed to process blocks", "target", target)
			s.clientsTried = 0 // reset the counter
			s.running.Store(false)
			return
		}
	}
	// rangeUpdate processed some blocks, re-evaluate the target and next block to continue indexing
	target = s.target
	next = s.nextNum()
	// reset the counter because we successfully processed some blocks with the current client
	s.clientsTried = 0
	// if the next block is within the target, we need to continue indexing
	if next <= s.target {
		s.log.Debug("More indexing needed, continuing", "target", target, "next", next)
		s.emitter.Emit(s.systemContext, superevents.ChainIndexingContinueEvent{
			ChainID: s.chain,
		})
		return
	}
	s.log.Debug("Idling indexing, reached latest block", "head", target)
	s.running.Store(false)
}

// nextActiveClient advances the client index and sets the active client.
func (s *ChainProcessor) nextActiveClient() {
	s.clientLock.Lock()
	defer s.clientLock.Unlock()
	if len(s.clients) == 0 {
		return
	}
	s.clientIndex = (s.clientIndex + 1) % len(s.clients)
	s.activeClient = s.clients[s.clientIndex]
	// track that we have advanced the client index
	s.clientsTried++
}

func (s *ChainProcessor) rangeUpdate(target uint64) (int, error) {
	s.clientLock.Lock()
	defer s.clientLock.Unlock()
	if len(s.clients) == 0 {
		return 0, types.ErrNoRPCSource
	}

	// define the range of blocks to fetch
	// [next, last] inclusive with a max of s.fetcherThreads blocks
	next := s.nextNum()
	last := target

	nums := make([]uint64, 0, s.maxFetcherThreads)
	for i := next; i <= last; i++ {
		nums = append(nums, i)
		// only attempt as many blocks as we can fetch in parallel
		if len(nums) >= s.maxFetcherThreads {
			s.log.Debug("Fetching up to max threads", "chain", s.chain.String(), "next", next, "last", last, "count", len(nums))
			break
		}
	}

	if len(nums) == 0 {
		s.log.Debug("No blocks to fetch", "chain", s.chain.String(), "next", next, "last", last)
		return 0, nil
	}

	s.log.Debug("Fetching blocks", "chain", s.chain.String(), "next", next, "last", last, "count", len(nums))

	// make a structure to receive parallel results
	type keyedResult struct {
		num      uint64
		blockRef *eth.BlockRef
		receipts gethtypes.Receipts
		err      error
	}
	parallelResults := make(chan keyedResult, len(nums))

	// each thread will fetch a block and its receipts and send the result to the channel
	fetch := func(wg *sync.WaitGroup, num uint64) {
		defer wg.Done()
		// ensure we emit the result at the end
		result := keyedResult{num, nil, nil, nil}
		defer func() { parallelResults <- result }()

		// fetch the block ref
		ctx, cancel := context.WithTimeout(s.systemContext, time.Second*10)
		nextL2BlockRef, err := s.activeClient.L2BlockRefByNumber(ctx, num)
		cancel()
		if err != nil {
			result.err = err
			return
		}
		next := nextL2BlockRef.BlockRef()
		if err := s.rewinder.AcceptedBlock(s.chain, next.ID()); err != nil {
			s.log.Warn("Cannot accept next block into events DB", "next", next.ID(), "err", err)
			result.err = err
			return
		}
		result.blockRef = &next

		// fetch receipts
		ctx, cancel = context.WithTimeout(s.systemContext, time.Second*10)
		receipts, err := s.activeClient.FetchReceipts(ctx, next.Hash)
		cancel()
		if err != nil {
			result.err = err
			return
		}
		result.receipts = receipts
	}

	// kick off the fetches and wait for them to complete
	var wg sync.WaitGroup
	for _, num := range nums {
		wg.Add(1)
		go fetch(&wg, num)
	}
	wg.Wait()

	// collect and sort the results
	results := make([]keyedResult, len(nums))
	for i := range nums {
		result := <-parallelResults
		results[i] = result
	}
	slices.SortFunc(results, func(a, b keyedResult) int {
		if a.num < b.num {
			return -1
		}
		if a.num > b.num {
			return 1
		}
		return 0
	})

	// process the results in order and return the first error encountered,
	// and the number of blocks processed successfully by this call
	for i := range results {
		if results[i].err != nil {
			return i, fmt.Errorf("failed to fetch block %d: %w", results[i].num, results[i].err)
		}
		// process the receipts
		err := s.process(s.systemContext, *results[i].blockRef, results[i].receipts)
		if err != nil {
			return i, fmt.Errorf("failed to process block %d: %w", results[i].num, err)
		}
	}
	return len(results), nil
}

func (s *ChainProcessor) process(ctx context.Context, next eth.BlockRef, receipts gethtypes.Receipts) error {
	if err := s.processor.ProcessLogs(ctx, next, receipts); err != nil {
		s.log.Error("Failed to process block", "block", next, "err", err)

		if next.Number == 0 { // cannot rewind genesis
			return nil
		}

		// Try to rewind the database to the previous block to remove any logs from this block that were written
		if err := s.rewinder.Rewind(s.chain, next.ParentID()); err != nil {
			// If any logs were written, our next attempt to write will fail and we'll retry this rewind.
			// If no logs were written successfully then the rewind wouldn't have done anything anyway.
			s.log.Error("Failed to rewind after error processing block", "block", next, "parent", next.ParentID(), "err", err)
		} else {
			s.log.Debug("Successfully rewound database to the previous block", "block", next, "parent", next.ParentID())
		}
		return err
	}
	s.log.Info("Indexed block events", "block", next, "txs", len(receipts))
	return nil
}
