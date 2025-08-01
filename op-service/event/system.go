package event

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/google/uuid"
)

type eventTraceKeyType struct{}

var (
	ctxKeyEventTrace = eventTraceKeyType{}
)

type eventTrace struct {
	UUID string
	Step int
}

func (e eventTrace) String() string {
	return fmt.Sprintf("%s:%d", e.UUID, e.Step)
}

type Registry interface {
	// Register registers a named event-emitter, optionally processing events itself:
	// deriver may be nil, not all registrants have to process events.
	// A non-nil deriver may implement AttachEmitter to automatically attach the Emitter to it,
	// before the deriver itself becomes executable.
	// A non-nil deriver may implement Unattacher to close resources upon being unregistered.
	Register(name string, deriver Deriver, opts ...RegisterOption) Emitter
	// Unregister removes a named emitter,
	// also removing it from the set of events-receiving derivers (if registered with non-nil deriver).
	// If the originally attached Deriver implements Unattacher it will be notified.
	Unregister(name string) (old Emitter)
}

type System interface {
	Registry
	// AddTracer registers a tracer to capture all event deriver/emitter work. It runs until RemoveTracer is called.
	// Duplicate tracers are allowed.
	AddTracer(t Tracer)
	// RemoveTracer removes a tracer. This is a no-op if the tracer was not previously added.
	// It will remove all added duplicates of the tracer.
	RemoveTracer(t Tracer)
	// Stop shuts down the System by un-registering all derivers/emitters.
	Stop()
}

type AttachEmitter interface {
	AttachEmitter(em Emitter)
}

// Unattacher is called when a deriver/emitter is unregistered from the system.
type Unattacher interface {
	Unattach()
}

type AnnotatedEvent struct {
	Ctx                 context.Context // Ctx passed in via Emit, and provided via executor to OnEvent handlers
	Event               Event
	EmitContext         uint64   // uniquely identifies the emission of the event, useful for debugging and creating diagrams
	EmitPriority        Priority // how important the emitter is, higher is more important
	PostProcessCallback func()   // callback to be called after the event is processed by all derivers
}

func (e AnnotatedEvent) Equals(other AnnotatedEvent) bool {
	return e.Event == other.Event && e.EmitContext == other.EmitContext && e.EmitPriority == other.EmitPriority
}

// systemActor is a deriver and/or emitter, registered in System with a name.
// If deriving, the actor is added as Executable to the Executor of the System.
type systemActor struct {
	name string
	sys  *Sys

	// To manage the execution peripherals, like rate-limiting, of this deriver
	ctx    context.Context
	cancel context.CancelFunc

	deriv         Deriver
	leaveExecutor func()

	// 0 if event does not originate from Deriver-handling of another event
	currentEvent uint64

	// How important this actor is as emitter. Higher is more important.
	// Emitted events from actors with a higher emit priority
	// will be prioritized over other queued up events.
	emitPriority Priority
}

func (r *systemActor) traceAndLogEventEmitted(ctx context.Context, level slog.Level, ev Event) context.Context {
	_, path, line, _ := runtime.Caller(2) // find the location of the caller of Emit()
	if strings.Contains(path, "limiter.go") {
		_, path, line, _ = runtime.Caller(3) // go one level up the stack to get the correct location, if the caller is rate-limited
	}

	file := filepath.Base(path)
	dir := filepath.Base(filepath.Dir(path))
	location := fmt.Sprintf("%s/%s:%d", dir, file, line)

	var etrace eventTrace
	if ctx.Value(ctxKeyEventTrace) == nil {
		etrace = eventTrace{
			UUID: uuid.New().String()[:6],
			Step: 0,
		}
		ctx = context.WithValue(ctx, ctxKeyEventTrace, etrace)
	} else {
		var ok bool
		etrace, ok = ctx.Value(ctxKeyEventTrace).(eventTrace)
		if !ok {
			r.sys.log.Error("Event trace is not a eventTrace type", "ev", ev, "loc", location)
			return ctx
		}
		etrace.Step++
		ctx = context.WithValue(ctx, ctxKeyEventTrace, etrace)
	}

	r.sys.log.Log(level, "Event emitted", "euid", etrace, "ev", ev, "loc", location)

	return ctx
}

// Emit is called by the end-user
func (r *systemActor) Emit(ctx context.Context, ev Event) {
	if ctx == nil {
		if testing.Testing() {
			panic(fmt.Errorf("emitter %s must provide a context with the emitted event %s", r.name, ev.String()))
		} else {
			// if not testing, then we will be more graceful, and allow the event to happen. The context may not be used.
			r.sys.log.Error("Event without context emitted", "emitter", r.name, "event", ev.String())
			ctx = context.Background()
		}
	}

	level := log.LevelTrace
	if r.sys.log.Enabled(ctx, level) {
		ctx = r.traceAndLogEventEmitted(ctx, level, ev)
	}

	if r.ctx.Err() != nil {
		return
	}
	r.sys.emit(r.name, r.currentEvent, ctx, ev, r.emitPriority)
}

// RunEvent is called by the events executor.
// While different things may execute in parallel, only one event is executed per entry at a time.
func (r *systemActor) RunEvent(ev AnnotatedEvent) {
	if r.deriv == nil {
		return
	}
	if r.ctx.Err() != nil {
		return
	}
	if r.sys.abort.Load() && !Is[CriticalErrorEvent](ev.Event) {
		// if aborting, and not the CriticalErrorEvent itself, then do not process the event
		return
	}

	prev := r.currentEvent
	start := time.Now()
	r.currentEvent = r.sys.recordDerivStart(r.name, ev, start)
	effect := r.deriv.OnEvent(ev.Ctx, ev.Event)
	elapsed := time.Since(start)
	r.sys.recordDerivEnd(r.name, ev, r.currentEvent, start, elapsed, effect)
	r.currentEvent = prev
}

// Sys is the canonical implementation of System.
type Sys struct {
	regs     map[string]*systemActor
	regsLock sync.Mutex

	log log.Logger

	executor Executor

	// used to generate a unique id for each event deriver processing call.
	derivContext atomic.Uint64
	// used to generate a unique id for each event-emission.
	emitContext atomic.Uint64

	tracers     []Tracer
	tracersLock sync.RWMutex

	// if true, no events may be processed, except CriticalError itself
	abort atomic.Bool
}

func NewSystem(log log.Logger, ex Executor) *Sys {
	return &Sys{
		regs:     make(map[string]*systemActor),
		executor: ex,
		log:      log,
	}
}

func (s *Sys) Register(name string, deriver Deriver, opts ...RegisterOption) Emitter {
	s.regsLock.Lock()
	defer s.regsLock.Unlock()

	if _, ok := s.regs[name]; ok {
		panic(fmt.Errorf("a deriver/emitter with name %q already exists", name))
	}

	cfg := defaultRegisterConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	ctx, cancel := context.WithCancel(context.Background())
	r := &systemActor{
		name:   name,
		deriv:  deriver,
		sys:    s,
		ctx:    ctx,
		cancel: cancel,
		// prioritize the outgoing messages
		emitPriority: cfg.Emitter.Priority,
	}
	s.regs[name] = r
	var em Emitter = r
	if cfg.Emitter.Limiting {
		limitedCallback := cfg.Emitter.OnLimited
		em = NewLimiter(ctx, r, cfg.Emitter.Rate, cfg.Emitter.Burst, func() {
			r.sys.recordRateLimited(name, r.currentEvent)
			if limitedCallback != nil {
				limitedCallback()
			}
		})
	}

	// If it can derive, add it to the executor (and only after attaching the emitter)
	if deriver != nil {
		// If it can emit, attach an emitter to it
		if attachTo, ok := deriver.(AttachEmitter); ok {
			attachTo.AttachEmitter(em)
		}
		r.leaveExecutor = s.executor.Add(r, &cfg.Executor)
	}
	return em
}

func (s *Sys) Unregister(name string) (previous Emitter) {
	s.regsLock.Lock()
	defer s.regsLock.Unlock()
	return s.unregister(name)
}

func (s *Sys) unregister(name string) (previous Emitter) {
	r, ok := s.regs[name]
	if !ok {
		return nil
	}
	r.cancel()
	// if this was registered as deriver with the executor, then leave the executor
	if r.leaveExecutor != nil {
		r.leaveExecutor()
	}
	delete(s.regs, name)
	if cl, ok := r.deriv.(Unattacher); ok {
		cl.Unattach()
	}
	return r
}

// Stop shuts down the system
// by unregistering all emitters/derivers,
// freeing up executor resources.
func (s *Sys) Stop() {
	s.regsLock.Lock()
	defer s.regsLock.Unlock()
	for _, r := range s.regs {
		s.unregister(r.name)
	}
}

func (s *Sys) AddTracer(t Tracer) {
	s.tracersLock.Lock()
	defer s.tracersLock.Unlock()
	s.tracers = append(s.tracers, t)
}

func (s *Sys) RemoveTracer(t Tracer) {
	s.tracersLock.Lock()
	defer s.tracersLock.Unlock()
	// We are not removing tracers often enough to optimize the deletion;
	// instead we prefer fast and simple tracer iteration during regular operation.
	s.tracers = slices.DeleteFunc(s.tracers, func(v Tracer) bool {
		return v == t
	})
}

// recordDeriv records that the deriver by name [deriv] is processing event [ev].
// This returns a unique integer (during lifetime of Sys), usable as ID to reference processing.
func (s *Sys) recordDerivStart(name string, ev AnnotatedEvent, startTime time.Time) uint64 {
	derivContext := s.derivContext.Add(1)

	s.tracersLock.RLock()
	defer s.tracersLock.RUnlock()
	for _, t := range s.tracers {
		t.OnDeriveStart(name, ev, derivContext, startTime)
	}

	return derivContext
}

func (s *Sys) recordDerivEnd(name string, ev AnnotatedEvent, derivContext uint64, startTime time.Time, duration time.Duration, effect bool) {
	s.tracersLock.RLock()
	defer s.tracersLock.RUnlock()
	for _, t := range s.tracers {
		t.OnDeriveEnd(name, ev, derivContext, startTime, duration, effect)
	}
}

func (s *Sys) recordRateLimited(name string, derivContext uint64) {
	s.tracersLock.RLock()
	defer s.tracersLock.RUnlock()
	s.log.Warn("Event-system emitter component was rate-limited", "emitter", name)
	for _, t := range s.tracers {
		t.OnRateLimited(name, derivContext)
	}
}

func (s *Sys) recordAfterProcessed(evtype string) {
	s.tracersLock.RLock()
	defer s.tracersLock.RUnlock()
	for _, t := range s.tracers {
		t.OnAfterProcessed(evtype)
	}
}

func (s *Sys) recordEmit(name string, ev AnnotatedEvent, derivContext uint64, emitTime time.Time) {
	s.tracersLock.RLock()
	defer s.tracersLock.RUnlock()
	for _, t := range s.tracers {
		t.OnEmit(name, ev, derivContext, emitTime)
	}
}

// emit an event [ev] during the derivation of another event, referenced by derivContext.
// If the event was emitted not as part of deriver event execution, then the derivContext is 0.
// The name of the emitter is provided to further contextualize the event.
func (s *Sys) emit(name string, derivContext uint64, ctx context.Context, ev Event, emitPriority Priority) {
	emitContext := s.emitContext.Add(1)
	annotated := AnnotatedEvent{
		Ctx:          ctx,
		Event:        ev,
		EmitContext:  emitContext,
		EmitPriority: emitPriority,
		PostProcessCallback: func() {
			s.recordAfterProcessed(ev.String())
		},
	}

	// As soon as anything emits a critical event,
	// make the system aware, before the executor event schedules it for processing.
	if Is[CriticalErrorEvent](ev) {
		s.abort.Store(true)
	}

	emitTime := time.Now()
	s.recordEmit(name, annotated, derivContext, emitTime)

	err := s.executor.Enqueue(annotated)
	// If the event system cannot enqueue an event, then it is a critical error
	// and we should panic to avoid deferred errors creating behaviors that are hard to reason about.
	// The Sys cannot decide if an event is important or not, so all events should be considered critical.
	if err != nil {
		s.log.Error("Failed to enqueue event", "emitter", name, "event", ev, "context", derivContext)
		panic(err)
	}
}
