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
	"github.com/tonkeeper/opentonapi/pkg/pusher/websocket"
)

// Server opens a port and exposes REST-ish API.
//
// Server integrates two groups of endpoints:
//  1. The first group named "Ogen" consists of endpoints generated by ogen based on api/openapi.yml.
//     It has an independent server in "oas" package.
//  2. The second group named "Async" contains methods that aren't supported by ogen (streaming methods with non-standard Content-Type).
//     These methods are defined manually and are exposed with http.ServeMux.
//
// We provide basic middleware like logging and metrics for both groups.
type Server struct {
	logger           *zap.Logger
	httpServer       *http.Server
	mux              *http.ServeMux
	asyncMiddlewares []AsyncMiddleware
}

type AsyncHandler func(http.ResponseWriter, *http.Request) error
type AsyncMiddleware func(AsyncHandler) AsyncHandler

type ServerOptions struct {
	ogenMiddlewares  []oas.Middleware
	asyncMiddlewares []AsyncMiddleware
	txSource         sources.TransactionSource
	liteServers      []config.LiteServer
}

type ServerOption func(options *ServerOptions)

func WithOgenMiddleware(m ...oas.Middleware) ServerOption {
	return func(options *ServerOptions) {
		options.ogenMiddlewares = m
	}
}

func WithAsyncMiddleware(m ...AsyncMiddleware) ServerOption {
	return func(options *ServerOptions) {
		options.asyncMiddlewares = m
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
	ogenMiddlewares := []oas.Middleware{ogenLoggingMiddleware(log), ogenMetricsMiddleware}
	ogenMiddlewares = append(ogenMiddlewares, options.ogenMiddlewares...)

	ogenServer, err := oas.NewServer(handler,
		oas.WithMiddleware(ogenMiddlewares...),
		oas.WithErrorHandler(ogenErrorsHandler))
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	asyncMiddlewares := []AsyncMiddleware{asyncLoggingMiddleware(log), asyncMetricsMiddleware}
	asyncMiddlewares = append(asyncMiddlewares, options.asyncMiddlewares...)

	sseHandler := sse.NewHandler(options.txSource)
	mux.Handle("/v2/sse/accounts/transactions", wrapAsync(chainMiddlewares(sse.Stream(sseHandler.SubscribeToTransactions), asyncMiddlewares...)))
	mux.Handle("/v2/websocket", wrapAsync(chainMiddlewares(websocket.Handler(log, options.txSource), asyncMiddlewares...)))
	mux.Handle("/", ogenServer)

	serv := Server{
		logger:           log,
		mux:              mux,
		asyncMiddlewares: asyncMiddlewares,
		httpServer: &http.Server{
			Addr:    address,
			Handler: mux,
		},
	}
	return &serv, nil
}

func wrapAsync(handler AsyncHandler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = handler(writer, request)
	})
}

func chainMiddlewares(handler AsyncHandler, middleware ...AsyncMiddleware) AsyncHandler {
	for _, md := range middleware {
		handler = md(handler)
	}
	return handler
}

func (s *Server) RegisterAsyncHandler(pattern string, handler AsyncHandler) {
	s.mux.Handle(pattern, wrapAsync(chainMiddlewares(handler, s.asyncMiddlewares...)))
}

func (s *Server) Run() {
	err := s.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		s.logger.Info("opentonapi quit")
		return
	}
	s.logger.Fatal("ListedAndServe() failed", zap.Error(err))
}
