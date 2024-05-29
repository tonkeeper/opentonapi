package sources

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/Narasimha1997/ratelimiter"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/tonkeeper/opentonapi/pkg/cache"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"go.uber.org/zap"
)

var (
	traceNumber = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "streaming_api_trace_dispatch",
		Help: "Number of traces",
	}, []string{"type"})
)

type SubscribeToTraceOptions struct {
	AllAccounts bool
	Accounts    []tongo.AccountID
}

type TraceSource interface {
	SubscribeToTraces(ctx context.Context, deliveryFn DeliveryFn, opts SubscribeToTraceOptions) CancelFn
}

type storage interface {
	GetTrace(ctx context.Context, hash tongo.Bits256) (*core.Trace, error)
}

type dispatcher interface {
	dispatch(accountIDs []tongo.AccountID, event []byte)
	RegisterSubscriber(fn DeliveryFn, options SubscribeToTraceOptions) CancelFn
}

type Tracer struct {
	logger     *zap.Logger
	storage    storage
	dispatcher dispatcher
	source     TraceSource

	// mu protects traceCache.
	// Tracer usually receives multiple tx hashes that are parts of the same trace,
	// and we want to avoid sending the same trace to subscribers multiple times.
	// We use a cache to keep track of already sent traces.
	// "cache.Cache" doesn't provide a single atomic operation to check and set a value if it hasn't been set yet,
	// so we use a mutex to serialize access to the cache.
	mu         sync.Mutex
	traceCache cache.Cache[string, struct{}]
}

func NewTracer(logger *zap.Logger, storage storage, source TraceSource) *Tracer {
	return &Tracer{
		logger:     logger,
		storage:    storage,
		source:     source,
		dispatcher: NewTraceDispatcher(logger),
		traceCache: cache.NewLRUCache[string, struct{}](10000, "tracer_trace_cache"),
	}
}

var _ TraceSource = (*Tracer)(nil)

func (t *Tracer) SubscribeToTraces(ctx context.Context, deliveryFn DeliveryFn, opts SubscribeToTraceOptions) CancelFn {
	t.logger.Debug("subscribe to traces",
		zap.Bool("all-accounts", opts.AllAccounts),
		zap.Stringers("accounts", opts.Accounts))

	return t.dispatcher.RegisterSubscriber(deliveryFn, opts)
}

func (t *Tracer) Run(ctx context.Context) {
	traceCh := make(chan TraceEvent, 1000)
	cancelFn := t.source.SubscribeToTraces(ctx, func(eventData []byte) {
		var tx TraceEvent
		if err := json.Unmarshal(eventData, &tx); err != nil {
			t.logger.Error("json.Unmarshal() failed", zap.Error(err))
			return
		}
		traceCh <- tx
	}, SubscribeToTraceOptions{AllAccounts: true})

	defer cancelFn()

	limiter := ratelimiter.NewDefaultLimiter(300, 10*time.Second)
	defer limiter.Kill()

	for txEvent := range traceCh {
		if ctx.Err() != nil {
			return
		}
		if allow, _ := limiter.ShouldAllow(1); !allow {
			traceNumber.With(map[string]string{"type": "dropped"}).Inc()
			continue
		}
		t.dispatch(txEvent)
	}
}

// putTraceInCache returns true if the trace was not in the cache before.
func (t *Tracer) putTraceInCache(hash string) (success bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, ok := t.traceCache.Get(hash); ok {
		return false
	}
	t.traceCache.Set(hash, struct{}{}, cache.WithExpiration(10*time.Minute))
	return true
}

func (t *Tracer) dispatch(trace TraceEvent) {
	traceNumber.With(map[string]string{"type": "dispatched-completed"}).Inc()

	if success := t.putTraceInCache(trace.ID.Hex()); !success {
		// ok, this trace is already in cache meaning that we already sent it to subscribers.
		// ignore it.
		return
	}
	traceNumber.With(map[string]string{"type": "converted-to-event"}).Inc()

	eventData := &TraceEventData{
		AccountIDs: trace.UniqueAccounts,
		Hash:       trace.ID.Hex(),
	}

	eventJSON, err := json.Marshal(eventData)
	if err != nil {
		t.logger.Error("json.Marshal() failed: %v", zap.Error(err))
		return
	}

	t.dispatcher.dispatch(trace.UniqueAccounts, eventJSON)
}
