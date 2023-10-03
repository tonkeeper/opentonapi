package sources

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
)

var (
	traceNumber = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "trace_number",
		Help: "Number of processed traces",
	}, []string{"completeness"})
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

type Tracer struct {
	logger     *zap.Logger
	storage    storage
	dispatcher *TraceDispatcher
	source     TransactionSource
}

func NewTracer(logger *zap.Logger, storage storage, source TransactionSource) *Tracer {
	return &Tracer{
		logger:     logger,
		storage:    storage,
		source:     source,
		dispatcher: NewTraceDispatcher(logger),
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
	num := 10 // todo: make configurable
	txCh := make(chan TransactionEventData, num)
	cancelFn := t.source.SubscribeToTransactions(ctx, func(eventData []byte) {
		var tx TransactionEventData
		if err := json.Unmarshal(eventData, &tx); err != nil {
			t.logger.Error("json.Unmarshal() failed", zap.Error(err))
			return
		}
		txCh <- tx
	}, SubscribeToTransactionsOptions{AllAccounts: true})

	defer cancelFn()

	var wg sync.WaitGroup
	wg.Add(num)

	for i := 0; i < num; i++ {
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case txEvent := <-txCh:
					var hash tongo.Bits256
					if err := hash.FromHex(txEvent.TxHash); err != nil {
						// this should never happen
						t.logger.Error("hash.FromHex() failed", zap.Error(err))
						continue
					}
					trace, err := t.storage.GetTrace(ctx, hash)
					if err != nil {
						continue
					}
					t.dispatch(trace)
				}
			}
		}()
	}
	wg.Wait()
}

func (t *Tracer) dispatch(trace *core.Trace) {
	if trace.InProgress() {
		traceNumber.With(map[string]string{"completeness": "inProgress"}).Inc()
		return
	}
	traceNumber.With(map[string]string{"completeness": "completed"}).Inc()

	accounts := make(map[tongo.AccountID]struct{})
	queue := []*core.Trace{trace}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		accounts[current.Account] = struct{}{}
		queue = append(queue, current.Children...)
	}

	accountIDs := maps.Keys(accounts)
	eventData := &TraceEventData{
		AccountIDs: accountIDs,
		Hash:       trace.Hash.Hex(),
	}
	eventJSON, err := json.Marshal(eventData)
	if err != nil {
		t.logger.Error("json.Marshal() failed: %v", zap.Error(err))
		return
	}

	t.dispatcher.dispatch(accountIDs, eventJSON)
}
