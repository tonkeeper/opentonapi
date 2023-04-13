package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/tonkeeper/tongo/config"
	"go.uber.org/zap"

	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/pusher/sources"
	"github.com/tonkeeper/opentonapi/pkg/pusher/sse"
)

type Server struct {
	logger     *zap.Logger
	httpServer *http.Server
}

type httpMiddleware func(http.Handler) http.Handler

type ServerOptions struct {
	middleware     []oas.Middleware
	httpMiddleware []httpMiddleware
	txSource       sources.TransactionSource
	liteServers    []config.LiteServer
}

type ServerOption func(options *ServerOptions)

func WithOasMiddleware(m ...oas.Middleware) ServerOption {
	return func(options *ServerOptions) {
		options.middleware = m
	}
}

func WithHttpMiddleware(m ...httpMiddleware) ServerOption {
	return func(options *ServerOptions) {
		options.httpMiddleware = m
	}
}

func WithTransactionSource(txSource sources.TransactionSource) ServerOption {
	return func(options *ServerOptions) {
		options.txSource = txSource
	}
}

func WithLiteServers(servers []config.LiteServer) ServerOption {
	return func(options *ServerOptions) {
		options.liteServers = servers
	}
}

func NewServer(log *zap.Logger, handler *Handler, address string, opts ...ServerOption) (*Server, error) {
	options := &ServerOptions{}
	for _, o := range opts {
		o(options)
	}
	if options.txSource == nil {
		s, err := sources.NewBlockchainSource(log, options.liteServers)
		if err != nil {
			return nil, err
		}
		go s.Run(context.TODO())

		options.txSource = s
	}
	middleware := []oas.Middleware{Logging(log), Metrics}
	middleware = append(middleware, options.middleware...)

	oasServer, err := oas.NewServer(handler, oas.WithMiddleware(middleware...), oas.WithErrorHandler(ErrorsHandler))
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()

	sseHandler := sse.NewHandler(options.txSource)
	mux.Handle("/v2/sse/accounts/transactions", applyMiddlewares(sseHandler.SubscribeToTransactions, options.httpMiddleware...))
	mux.Handle("/", oasServer)

	serv := Server{
		logger: log,
		httpServer: &http.Server{
			Addr:    address,
			Handler: mux,
		},
	}
	return &serv, nil
}

func (s *Server) Run() {
	err := s.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		s.logger.Info("opentonapi quit")
		return
	}
	s.logger.Fatal("ListedAndServe() failed", zap.Error(err))
}

func applyMiddlewares(handlerFunc sse.HandlerFunc, middleware ...httpMiddleware) http.Handler {
	handler := sse.StreamingMiddleware(handlerFunc)
	for _, md := range middleware {
		handler = md(handler)
	}
	return handler
}
